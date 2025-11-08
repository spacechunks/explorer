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
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/controlplane/user"
)

type Service interface {
	GetInstance(ctx context.Context, id string) (Instance, error)
	ListInstances(ctx context.Context) ([]Instance, error)
	RunFlavorVersion(ctx context.Context, chunkID string, flavorVersionID string) (Instance, error)
	DiscoverInstances(ctx context.Context, nodeID string) ([]Instance, error)
	ReceiveInstanceStatusReports(ctx context.Context, reports []StatusReport) error
}

type svc struct {
	logger       *slog.Logger
	insRepo      Repository
	nodeRepo     node.Repository
	chunkService chunk.Service
}

func NewService(logger *slog.Logger, insRepo Repository, nodeRepo node.Repository, chunkService chunk.Service) Service {
	return &svc{
		logger:       logger,
		insRepo:      insRepo,
		nodeRepo:     nodeRepo,
		chunkService: chunkService,
	}
}

func (s *svc) GetInstance(ctx context.Context, id string) (Instance, error) {
	ins, err := s.insRepo.GetInstanceByID(ctx, id)
	if err != nil {
		return Instance{}, err
	}
	return ins, nil
}

func (s *svc) ListInstances(ctx context.Context) ([]Instance, error) {
	l, err := s.insRepo.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (s *svc) RunFlavorVersion(ctx context.Context, chunkID string, flavorVersionID string) (Instance, error) {
	// TODO: at some point implement a more sophisticated node scheduling logic
	n, err := s.nodeRepo.RandomNode(ctx)
	if err != nil {
		return Instance{}, fmt.Errorf("random node: %w", err)
	}

	c, err := s.chunkService.GetChunk(ctx, chunkID)
	if err != nil {
		return Instance{}, fmt.Errorf("chunk by id: %w", err)
	}

	versions := make(map[string]chunk.FlavorVersion)

	for _, f := range c.Flavors {
		for _, v := range f.Versions {
			versions[v.ID] = v
		}
	}

	ver, ok := versions[flavorVersionID]
	if !ok {
		return Instance{}, apierrs.ErrFlavorVersionNotFound
	}

	instanceID, err := uuid.NewV7()
	if err != nil {
		return Instance{}, fmt.Errorf("instance id: %w", err)
	}

	ins, err := s.insRepo.CreateInstance(ctx, Instance{
		ID:            instanceID.String(),
		Chunk:         c,
		FlavorVersion: ver,
		State:         StatePending,
		Owner:         user.User{}, // TODO: get user from api tok
	}, n.ID)
	if err != nil {
		return Instance{}, fmt.Errorf("create instance: %w", err)
	}

	return ins, nil
}

func (s *svc) DiscoverInstances(ctx context.Context, nodeID string) ([]Instance, error) {
	instances, err := s.insRepo.GetInstancesByNodeID(ctx, nodeID)
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
	if err := s.insRepo.ApplyStatusReports(ctx, reports); err != nil {
		return fmt.Errorf("apply status reports: %w", err)
	}
	return nil
}

// TODO: tests
