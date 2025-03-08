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
	"fmt"
	"log/slog"
	"time"

	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/platformd/workload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Syncer is responsible for syncing the state found in the control plane
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
//     -> if workload already exists, do nothing
//
//     -> if workload is already gone, set status [workload.StateDeleted]
type Syncer struct {
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

func NewSyncer(
	logger *slog.Logger,
	cfg syncerConfig,
	insClient instancev1alpha1.InstanceServiceClient,
	wlService workload.Service,
	store workload.StatusStore,
) Syncer {
	return Syncer{
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

func (s *Syncer) Start(ctx context.Context) {
	for {
		select {
		case <-s.ticker.C:
			s.tick(ctx)
		case <-s.stop:
			return
		}
	}
}

func (s *Syncer) Stop() {
	s.ticker.Stop()
	s.stop <- true
}

func (s *Syncer) tick(ctx context.Context) {
	resp, err := s.insClient.DiscoverInstances(ctx, &instancev1alpha1.DiscoverInstanceRequest{
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

	for _, ins := range resp.Instances {
		switch ins.GetState() {
		case instancev1alpha1.InstanceState_PENDING:
			s.handlePendingInstance(ctx, ins)
			continue
		// DELETING is set by the control plane once an instance
		// should be stopped and removed
		case instancev1alpha1.InstanceState_DELETING:
			s.handleInstanceStopped(ctx, ins)
			continue
		default:
		}
	}

	// set the sync interval again, in case we errored before
	s.ticker.Reset(s.cfg.SyncInterval)
}

func (s *Syncer) handlePendingInstance(ctx context.Context, instance *instancev1alpha1.Instance) {
	var (
		id      = instance.GetId()
		attempt = s.attempts[id]
	)

	// if there is already a status present for this instance
	// skip creation, because we can assume that it has already
	// been created. if calling RunWorkload fails, no status
	// will be stored. this enables a retry.
	//
	// this check is necessary, because it can happen that we
	// receive an instance that is in RUNNING state from the
	// control plane, even though we already called RunWorkload,
	// because the state update did not reach the control plane
	// yet.
	if s.store.Get(id) != nil {
		return
	}

	if attempt >= s.cfg.MaxAttempts {
		s.logger.WarnContext(ctx, "max attempts reached", "instance_id", id, "attempt", attempt)
		w := &workload.Workload{
			ID: id,
		}

		s.store.Update(w.ID, workload.Status{
			State: workload.StateCreationFailed,
		})
		return
	}

	port, err := s.portAlloc.Allocate()
	if err != nil {
		s.attempts[id] = attempt + 1
		slog.ErrorContext(
			ctx,
			"failed to allocate port",
			"instance_id", id,
			"attempt", s.attempts[id],
			"err", err,
		)
		return
	}

	wStatus := workload.Status{
		State: workload.StateCreating,
		Port:  port,
	}

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
		Labels:    workload.InstanceLabels(instance),
		Status:    wStatus,
	}

	if err := s.wlService.RunWorkload(ctx, w); err != nil {
		s.attempts[id] = attempt + 1
		s.logger.ErrorContext(
			ctx,
			"failed to create workload",
			"instance_id", id,
			"attempt", s.attempts[id],
			"err", err,
		)

		// very important to free the allocated port here, because
		// if we exceed the maximum amount of attempts, the port
		// will stay allocated as we return the function.
		s.portAlloc.Free(port)
		return
	}

	s.store.Update(id, wStatus)
}

func (s *Syncer) handleInstanceStopped(ctx context.Context, instance *instancev1alpha1.Instance) {
	if err := s.wlService.RemoveWorkload(ctx, instance.GetId()); err != nil {
		st, ok := status.FromError(err)
		if !ok {
			slog.ErrorContext(ctx, "failed to remove workload", "instance_id", instance.GetId(), "err", err)
			return
		}
		// not found is not a critical error, we need to be notified about
		if st.Code() != codes.NotFound {
			slog.ErrorContext(ctx, "failed to remove workload", "instance_id", instance.GetId(), "err", err)
			return
		}
	}

	s.store.Update(instance.GetId(), workload.Status{
		State: workload.StateDeleted,
	})
}
