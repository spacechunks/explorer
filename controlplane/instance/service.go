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
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/spacechunks/explorer/controlplane/chunk"
)

type Service interface {
	RunChunk(ctx context.Context, chunkID string, flavorID string) (Instance, error)
	DiscoverInstances(ctx context.Context, nodeID string) ([]Instance, error)
	ReceiveInstanceStatusReports(ctx context.Context, reports []StatusReport) error
}

type svc struct {
	logger       *slog.Logger
	repo         Repository
	chunkService chunk.Service
}

func NewService(logger *slog.Logger, repo Repository, chunkService chunk.Service) Service {
	return &svc{
		logger:       logger,
		repo:         repo,
		chunkService: chunkService,
	}
}

func (s *svc) RunChunk(ctx context.Context, chunkID string, flavorID string) (Instance, error) {
	// FIXME: hardcoded for now, determine node to schedule instance to later
	const nodeID = "0195c2f6-f40c-72df-a0f1-e468f1be77b1"

	c, err := s.chunkService.GetChunk(ctx, chunkID)
	if err != nil {
		return Instance{}, fmt.Errorf("chunk by id: %w", err)
	}

	var flavor *chunk.Flavor
	for _, f := range c.Flavors {
		if f.ID == flavorID {
			flavor = &f
			break
		}
	}

	if flavor == nil {
		return Instance{}, fmt.Errorf("flavor not found")
	}

	instanceID, err := uuid.NewV7()
	if err != nil {
		return Instance{}, fmt.Errorf("instance id: %w", err)
	}

	ins, err := s.repo.CreateInstance(ctx, Instance{
		ID:          instanceID.String(),
		Chunk:       c,
		ChunkFlavor: *flavor,
		State:       StatePending,
	}, nodeID)
	if err != nil {
		return Instance{}, fmt.Errorf("create instance: %w", err)
	}

	return ins, nil
}

func (s *svc) DiscoverInstances(ctx context.Context, nodeID string) ([]Instance, error) {
	instances, err := s.repo.GetInstancesByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	s.logger.DebugContext(ctx, "found instances", "instances", len(instances), "node_id", nodeID)

	// sort to return consistent output
	sort.Slice(instances, func(i, j int) bool {
		return strings.Compare(instances[i].ID, instances[j].ID) < 0
	})

	return instances, nil
}

func (s *svc) ReceiveInstanceStatusReports(ctx context.Context, reports []StatusReport) error {
	if err := s.repo.ApplyStatusReports(ctx, reports); err != nil {
		return fmt.Errorf("apply status reports: %w", err)
	}
	return nil
}

// TODO: tests
