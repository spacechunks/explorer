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
	"log/slog"
	"time"

	"github.com/spacechunks/explorer/platformd/workload"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type podGC struct {
	logger   *slog.Logger
	attempts uint32
	rtClient runtimev1.RuntimeServiceClient
	ticker   *time.Ticker
	stop     chan bool
}

func newPodGC(
	logger *slog.Logger,
	client runtimev1.RuntimeServiceClient,
	interval time.Duration,
	attemptThreshold uint32,
) podGC {
	return podGC{
		logger:   logger.With("component", "pod-gc"),
		attempts: attemptThreshold,
		rtClient: client,
		ticker:   time.NewTicker(interval),
		stop:     make(chan bool),
	}
}

func (p *podGC) Start(ctx context.Context) {
	for {
		select {
		case <-p.ticker.C:
			resp, err := p.rtClient.ListPodSandbox(ctx, &runtimev1.ListPodSandboxRequest{
				Filter: &runtimev1.PodSandboxFilter{
					LabelSelector: map[string]string{
						workload.LabelWorkloadType: "instance",
					},
				},
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed listing pod sandboxes", "err", err)
				continue
			}
			for _, pod := range resp.Items {
				if pod.Metadata.Attempt < p.attempts {
					continue
				}
				if _, err := p.rtClient.RemovePodSandbox(ctx, &runtimev1.RemovePodSandboxRequest{
					PodSandboxId: pod.Id,
				}); err != nil {
					slog.ErrorContext(
						ctx,
						"failed removing pod sandbox",
						"pod_id", pod.Id,
						"pod_name", pod.GetMetadata().GetName(),
						"err", err,
					)
				}
			}
		case <-p.stop:
			return
		}
	}
}

func (p *podGC) Stop() {
	p.ticker.Stop()
	p.stop <- true
}
