/*
 Explorer Platform, a platform for hosting and discovering Minecraft servers.
 Copyright (C) 2024 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package platformd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"time"

	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/platformd/workload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errMaxAttemptsReached = errors.New("syncer: max attempts reached")

// syncer is responsible for syncing the state found in the control plane
// with what is currently running on the CRI side. basically, it does the
// following:
//
//   - fetch all instances from control plane
//
//   - instances with state [instancev1alpha1.InstanceState_PENDING]
//
//     -> try to create the workload
//
//     -> record attempts for creating the workload
//
//     -> if maximum number of attempts is reached set state to [workload.StateCreationFailed]
//
//   - instances with state [instancev1alpha1.InstanceState_DELETING]:
//
//     -> try to remove the workload
//
//     -> if the workload is already gone, set status to [workload.StateDeleted]
//
//   - instances with state [instancev1alpha1.InstanceState_RUNNING]:
//
//     -> if workload is running, do nothing
//
//     -> if workload is already gone, set status [workload.StateDeleted]
//
//     -> if workload container has status exited or unknown, remove it
type syncer struct {
	logger *slog.Logger

	cfg syncerConfig

	insClient instancev1alpha1.InstanceServiceClient
	wlService workload.Service
	store     workload.StatusStore
	portAlloc *portAllocator

	attempts map[string]int
	ticker   *time.Ticker
	stop     chan bool
}

type syncerConfig struct {
	MaxAttempts       int
	SyncInterval      time.Duration
	NodeID            string
	MinPort           uint16
	MaxPort           uint16
	WorkloadNamespace string
	RegistryEndpoint  string
}

func newSyncer(
	logger *slog.Logger,
	cfg syncerConfig,
	insClient instancev1alpha1.InstanceServiceClient,
	wlService workload.Service,
	store workload.StatusStore,
) syncer {
	return syncer{
		logger:    logger.With("component", "syncer"),
		cfg:       cfg,
		insClient: insClient,
		wlService: wlService,
		store:     store,
		portAlloc: newPortAllocator(cfg.MinPort, cfg.MaxPort),
		ticker:    time.NewTicker(cfg.SyncInterval),
		attempts:  make(map[string]int),
		stop:      make(chan bool),
	}
}

func (s *syncer) Start(ctx context.Context) {
	for {
		select {
		case <-s.ticker.C:
			s.tick(ctx)
		case <-s.stop:
			return
		}
	}
}

func (s *syncer) Stop() {
	s.ticker.Stop()
	s.stop <- true
}

func (s *syncer) tick(ctx context.Context) {
	discResp, err := s.insClient.DiscoverInstances(ctx, &instancev1alpha1.DiscoverInstanceRequest{
		NodeKey: &s.cfg.NodeID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "discover instances failed", "node_id", s.cfg.NodeID, "err", err)
		// if we encounter an error backoff for a longer period than the sync interval,
		// because we don't want to spam the control plane if things are not working.
		// FIXME: use exponential backoff
		s.ticker.Reset(3 * time.Second)
		return
	}

	for _, ins := range discResp.Instances {
		id := ins.GetId()
		switch ins.GetState() {
		case instancev1alpha1.InstanceState_PENDING:
			if err := s.handleInstancePending(ctx, ins); err != nil {
				if errors.Is(err, errMaxAttemptsReached) {
					s.logger.WarnContext(ctx, "max attempts reached", "instance_id", id, "attempt", s.attempts[id])
					continue
				}
				s.attempts[id] = s.attempts[id] + 1
				s.logger.ErrorContext(ctx, "failed to run workload", "instance_id", id, "attempt", s.attempts[id], "err", err)
			}
			continue
		// DELETING is set by the control plane once an instance
		// should be stopped and removed
		case instancev1alpha1.InstanceState_DELETING:
			if err := s.handleInstanceDeleting(ctx, ins); err != nil {
				s.logger.ErrorContext(ctx, "failed to delete instance", "instance_id", id)
			}
			continue
		case instancev1alpha1.InstanceState_RUNNING:
			if err := s.handleInstanceRunning(ctx, ins); err != nil {
				s.logger.ErrorContext(ctx, "handling a running instance failed", "instance_id", id, "err", err)
			}
			continue
		default:
			s.logger.DebugContext(ctx, "skipping instance", "state", ins.GetState())
			continue
		}
	}

	// TODO: report state back to control plane
	// once reported remove DELETED and CREATION_FAILED entries from store

	// set the sync interval again, in case we errored before
	s.ticker.Reset(s.cfg.SyncInterval)
}

func (s *syncer) handleInstancePending(ctx context.Context, instance *instancev1alpha1.Instance) error {
	var (
		id      = instance.GetId()
		attempt = s.attempts[id]
	)

	// if the instance is not in state CREATING, skip it.
	// this check is necessary, because it can happen that we
	// receive an instance that is still in PENDING state from the
	// control plane, even though we already called RunWorkload,
	// successfully, because the state update did not reach
	// the control plane yet or a bug in the control plane does
	// not update states correctly.
	if stat := s.store.Get(id); stat != nil && stat.State != workload.StateCreating {
		return nil
	}

	if attempt >= s.cfg.MaxAttempts {
		s.store.Update(id, workload.Status{
			State: workload.StateCreationFailed,
		})
		return errMaxAttemptsReached
	}

	attempt++

	s.store.Update(id, workload.Status{
		State: workload.StateCreating,
	})

	port, err := s.portAlloc.Allocate()
	if err != nil {
		return fmt.Errorf("failed to allocate port: %w", err)
	}

	var (
		wStatus = workload.Status{
			State: workload.StateCreating,
			Port:  port,
		}
		labels = map[string]string{
			workload.LabelWorkloadPort: strconv.Itoa(int(port)),
		}
	)

	maps.Copy(labels, workload.InstanceLabels(instance))

	w := workload.Workload{
		ID:   id,
		Name: instance.GetChunk().GetName() + "_" + instance.GetFlavor().GetName(),
		Image: fmt.Sprintf(
			"%s/%s/%s",
			s.cfg.RegistryEndpoint,
			instance.GetChunk().GetName(),
			instance.GetFlavor().GetName(),
		),
		Namespace: s.cfg.WorkloadNamespace,
		Hostname:  id,
		Labels:    labels,
	}

	if err := s.wlService.RunWorkload(ctx, w, attempt); err != nil {
		// very important to free the allocated port here, because
		// if we exceed the maximum amount of attempts, the port
		// will stay allocated as we return the function.
		s.portAlloc.Free(port)

		// FIXME: with this implementation we waste a whole attempt
		//        when the workload cannot be removed in this step.
		//        fix this by checking if a pod exists before running
		//        and cleaning it up.

		// clean up the not working state, so we can retry
		// with a fresh pod.
		if err := s.wlService.RemoveWorkload(ctx, id); err != nil {
			slog.ErrorContext(ctx, "failed to remove workload", "instance_id", id, "err", err)
		}

		return fmt.Errorf("run workload: %w", err)
	}

	wStatus.State = workload.StateRunning
	s.store.Update(id, wStatus)
	return nil
}

func (s *syncer) handleInstanceDeleting(ctx context.Context, instance *instancev1alpha1.Instance) error {
	if err := s.wlService.RemoveWorkload(ctx, instance.GetId()); err != nil {
		if isNotFound(err) {
			s.store.Update(instance.GetId(), workload.Status{
				State: workload.StateDeleted,
			})
			return nil
		}
		return fmt.Errorf("remove workload: %w", err)
	}

	s.store.Update(instance.GetId(), workload.Status{
		State: workload.StateDeleted,
	})

	return nil
}

func (s *syncer) handleInstanceRunning(ctx context.Context, instance *instancev1alpha1.Instance) error {
	health, err := s.wlService.GetWorkloadHealth(ctx, instance.GetId())
	if err != nil {
		return fmt.Errorf("get workload health: %w", err)
	}

	if health == workload.HealthStatusHealthy {
		return nil
	}

	if err := s.wlService.RemoveWorkload(ctx, instance.GetId()); err != nil {
		// can happen if control plane update fails, but workload has already been deleted.
		// this also enables manual deletion of pods, can come in handy when debugging.
		if isNotFound(err) {
			s.store.Update(instance.GetId(), workload.Status{
				State: workload.StateDeleted,
			})
			return nil
		}
		return fmt.Errorf("remove workload: %w", err)
	}

	s.store.Update(instance.GetId(), workload.Status{
		State: workload.StateDeleted,
	})

	return nil
}

func isNotFound(err error) bool {
	st, ok := status.FromError(err)
	if ok && st.Code() == codes.NotFound {
		return true
	}
	return false
}
