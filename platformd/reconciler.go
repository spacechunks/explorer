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
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/platformd/workload"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

var errMaxAttemptsReached = errors.New("reconciler: max attempts reached")

// reconciler is responsible for syncing the state found in the control plane
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
type reconciler struct {
	logger *slog.Logger

	cfg reconcilerConfig

	insClient instancev1alpha1.InstanceServiceClient
	wlService workload.Service
	store     workload.StatusStore
	portAlloc *portAllocator

	attempts map[string]uint
	ticker   *time.Ticker
	stop     chan bool
}

type reconcilerConfig struct {
	MaxAttempts         uint
	SyncInterval        time.Duration
	NodeID              string
	MinPort             uint16
	MaxPort             uint16
	WorkloadNamespace   string
	WorkloadCPUPeriod   uint64
	WorkloadCPUQuota    uint64
	WorkloadMemoryLimit uint64
	RegistryEndpoint    string
}

func newReconciler(
	logger *slog.Logger,
	cfg reconcilerConfig,
	insClient instancev1alpha1.InstanceServiceClient,
	wlService workload.Service,
	store workload.StatusStore,
) reconciler {
	return reconciler{
		logger:    logger.With("component", "reconciler"),
		cfg:       cfg,
		insClient: insClient,
		wlService: wlService,
		store:     store,
		portAlloc: newPortAllocator(cfg.MinPort, cfg.MaxPort),
		ticker:    time.NewTicker(cfg.SyncInterval),
		attempts:  make(map[string]uint),
		stop:      make(chan bool),
	}
}

func (r *reconciler) Start(ctx context.Context) {
	for {
		select {
		case <-r.ticker.C:
			r.tick(ctx)
		case <-r.stop:
			return
		}
	}
}

func (r *reconciler) Stop() {
	r.ticker.Stop()
	r.stop <- true
}

func (r *reconciler) tick(ctx context.Context) {
	discResp, err := r.insClient.DiscoverInstances(ctx, &instancev1alpha1.DiscoverInstanceRequest{
		NodeKey: &r.cfg.NodeID,
	})
	if err != nil {
		r.logger.ErrorContext(ctx, "discover instances failed", "node_id", r.cfg.NodeID, "err", err)
		// if we encounter an error backoff for a longer period than the sync interval,
		// because we don't want to spam the control plane if things are not working.
		// FIXME: use exponential backoff
		r.ticker.Reset(3 * time.Second)
		return
	}

	for _, ins := range discResp.Instances {
		id := ins.GetId()
		switch ins.GetState() {
		case instancev1alpha1.InstanceState_PENDING, instancev1alpha1.InstanceState_CREATING:
			if err := r.handleInstanceCreation(ctx, ins); err != nil {
				if errors.Is(err, errMaxAttemptsReached) {
					r.logger.WarnContext(ctx,
						"max attempts reached",
						"instance_id", id,
						"attempt", r.attempts[id],
					)
					continue
				}
				r.attempts[id] = r.attempts[id] + 1
				r.logger.ErrorContext(ctx,
					"failed to run workload",
					"instance_id", id,
					"attempt", r.attempts[id],
					"err", err,
				)
			}
			continue
		// DELETING is set by the control plane once an instance
		// should be stopped and removed
		case instancev1alpha1.InstanceState_DELETING:
			if err := r.handleInstanceDeleting(ctx, ins); err != nil {
				r.logger.ErrorContext(ctx, "failed to delete instance", "instance_id", id)
			}
			continue
		case instancev1alpha1.InstanceState_RUNNING:
			if err := r.handleInstanceRunning(ctx, ins); err != nil {
				r.logger.ErrorContext(ctx,
					"handling a running instance failed",
					"instance_id", id,
					"err", err,
				)
			}
			continue
		default:
			r.logger.InfoContext(ctx, "skipping instance", "state", ins.GetState()) // TODO: debug
			continue
		}
	}

	var (
		statuses = r.store.View()
		items    = make([]*instancev1alpha1.InstanceStatusReport, 0, len(statuses))
	)

	for k, v := range statuses {
		r.logger.InfoContext(ctx, "instance status", "instance_id", k, "state", v.State, "port", v.Port)
		items = append(items, &instancev1alpha1.InstanceStatusReport{
			InstanceId: &k,
			Port:       ptr.Pointer(uint32(v.Port)),
			State: ptr.Pointer(
				instancev1alpha1.InstanceState(instancev1alpha1.InstanceState_value[string(v.State)]),
			),
		})
	}

	if _, err := r.insClient.ReceiveInstanceStatusReports(ctx, &instancev1alpha1.ReceiveInstanceStatusReportsRequest{
		Reports: items,
	}); err != nil {
		r.logger.ErrorContext(ctx, "sending workload status reports failed", "err", err)
		r.ticker.Reset(3 * time.Second)
		return
	}

	for k, v := range statuses {
		if v.State == workload.StateDeleted || v.State == workload.StateCreationFailed {
			r.store.Del(k)
		}
	}

	// set the sync interval again, in case we errored before
	r.ticker.Reset(r.cfg.SyncInterval)
}

func (r *reconciler) handleInstanceCreation(ctx context.Context, instance *instancev1alpha1.Instance) error {
	var (
		id      = instance.GetId()
		attempt = r.attempts[id]
	)

	r.logger.InfoContext(ctx, "handling pending instance", "instance_id", id, "attempt", attempt)

	// if the instance is not in state CREATING, skip it.
	// this check is necessary, because it can happen that we
	// receive an instance that is still in PENDING state from the
	// control plane, even though we already called RunWorkload,
	// successfully, because the state update did not reach
	// the control plane yet or a bug in the control plane does
	// not update states correctly.
	if status := r.store.Get(id); status != nil && status.State != workload.StateCreating {
		r.logger.InfoContext(ctx, "skip")
		return nil
	}

	if attempt >= r.cfg.MaxAttempts {
		r.store.Update(id, workload.Status{
			State: workload.StateCreationFailed,
		})
		return errMaxAttemptsReached
	}

	attempt++

	r.store.Update(id, workload.Status{
		State: workload.StateCreating,
	})

	port, err := r.portAlloc.Allocate()
	if err != nil {
		return fmt.Errorf("failed to allocate port: %w", err)
	}

	var (
		status = workload.Status{
			State: workload.StateCreating,
			Port:  port,
		}
		baseURL = fmt.Sprintf(
			"%s/%s/%s",
			r.cfg.RegistryEndpoint,
			instance.GetChunk().GetName(),
			instance.GetFlavor().GetName(),
		)
		labels = map[string]string{
			workload.LabelWorkloadPort: strconv.Itoa(int(port)),
		}
	)

	maps.Copy(labels, workload.InstanceLabels(instance))

	w := workload.Workload{
		ID:               id,
		Name:             instance.GetChunk().GetName() + "_" + instance.GetFlavor().GetName(),
		BaseImage:        baseURL + "/base",
		CheckpointImage:  baseURL + "/checkpoint",
		Namespace:        r.cfg.WorkloadNamespace,
		Hostname:         id,
		Labels:           labels,
		CPUPeriod:        r.cfg.WorkloadCPUPeriod,
		CPUQuota:         r.cfg.WorkloadCPUQuota,
		MemoryLimitBytes: r.cfg.WorkloadMemoryLimit,
	}

	if err := r.wlService.RunWorkload(ctx, w, attempt); err != nil {
		// very important to free the allocated port here, because
		// if we exceed the maximum amount of attempts, the port
		// will stay allocated as we return the function.
		r.portAlloc.Free(port)

		// FIXME: with this implementation we waste a whole attempt
		//        when the workload cannot be removed in this step.
		//        fix this by checking if a pod exists before running
		//        and cleaning it up.

		// clean up the not working state, so we can retry
		// with a fresh pod.
		if err := r.wlService.RemoveWorkload(ctx, id); err != nil {
			slog.ErrorContext(ctx, "failed to remove workload", "instance_id", id, "err", err)
		}

		return fmt.Errorf("run workload: %w", err)
	}

	status.State = workload.StateRunning
	r.store.Update(id, status)
	return nil
}

func (r *reconciler) handleInstanceDeleting(ctx context.Context, instance *instancev1alpha1.Instance) error {
	if err := r.wlService.RemoveWorkload(ctx, instance.GetId()); err != nil {
		if isNotFound(err) {
			r.store.Update(instance.GetId(), workload.Status{
				State: workload.StateDeleted,
			})
			return nil
		}
		return fmt.Errorf("remove workload: %w", err)
	}

	r.store.Update(instance.GetId(), workload.Status{
		State: workload.StateDeleted,
	})

	return nil
}

func (r *reconciler) handleInstanceRunning(ctx context.Context, instance *instancev1alpha1.Instance) error {
	health, err := r.wlService.GetWorkloadHealth(ctx, instance.GetId())
	if err != nil {
		return fmt.Errorf("get workload health: %w", err)
	}

	if health == workload.HealthStatusHealthy {
		return nil
	}

	if err := r.wlService.RemoveWorkload(ctx, instance.GetId()); err != nil {
		// can happen if control plane update fails, but workload has already been deleted.
		// this also enables manual deletion of pods, can come in handy when debugging.
		if isNotFound(err) {
			r.store.Update(instance.GetId(), workload.Status{
				State: workload.StateDeleted,
			})
			return nil
		}
		return fmt.Errorf("remove workload: %w", err)
	}

	r.store.Update(instance.GetId(), workload.Status{
		State: workload.StateDeleted,
	})

	return nil
}

func isNotFound(err error) bool {
	st, ok := grpcstatus.FromError(err)
	if ok && st.Code() == codes.NotFound {
		return true
	}
	return false
}
