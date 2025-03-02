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

package instance

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/spacechunks/explorer/api/platformd/workload/v1alpha2"
)

type Service interface {
	RunChunk(ctx context.Context, id string) (Instance, error)
	DiscoverInstances(ctx context.Context, nodeID string) ([]Instance, error)
}

type svc struct {
	logger *slog.Logger
	repo   Repository
}

func NewService(logger *slog.Logger, repo Repository) Service {
	return &svc{
		logger: logger,
		repo:   repo,
	}
}

func (s *svc) RunChunk(ctx context.Context, id string) (Instance, error) {
	/*
		c, err := s.repo.GetChunkByID(ctx, id)
		if err != nil {
			return resource.Instance{}, fmt.Errorf("chunk by id: %w", err)
		}

		// TODO: determine node to schedule instance to

		instanceID, err := uuid.NewV7()
		if err != nil {
			return resource.Instance{}, fmt.Errorf("instance id: %w", err)
		}

		ins, err := s.repo.CreateInstance(ctx, resource.Instance{
			ID:    instanceID.String(),
			Chunk: c,
		}, "")
		if err != nil {
			return Instance{}, fmt.Errorf("create instance: %w", err)
		}*/
	return Instance{}, nil
}

func (s *svc) DiscoverInstances(ctx context.Context, nodeID string) ([]Instance, error) {
	instances, err := s.repo.GetInstancesByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "found instances", "instances", len(instances), "node_id", nodeID)

	// sort to return consistent output
	sort.Slice(instances, func(i, j int) bool {
		return strings.Compare(instances[i].ID, instances[j].ID) < 0
	})

	return instances, nil
}

func (s *svc) ReceiveWorkloadStateReports(status []v1alpha2.WorkloadStatus) error {
	// TODO:
	// * update instance state based on workload state
	//   * if workload state == DELETED
	//     * remove from table

	return nil
}
