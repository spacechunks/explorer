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
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spacechunks/explorer/controlplane/chunk"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/internal/resource"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Service interface {
	GetInstance(ctx context.Context, id string) (resource.Instance, error)
	ListInstances(ctx context.Context, pageSize int, afterID *string) ([]resource.Instance, error)
	RunFlavorVersion(
		ctx context.Context,
		flavorVersionID string,
		ownerID string,
		orderedBy string,
	) (resource.Instance, error)
	DiscoverInstances(ctx context.Context, nodeID string) ([]resource.Instance, error)
	ReceiveInstanceStatusReports(ctx context.Context, reports []resource.InstanceStatusReport) error
}

type svc struct {
	logger    *slog.Logger
	insRepo   Repository
	nodeRepo  node.Repository
	chunkRepo chunk.Repository
	metrics   metrics
}

func NewService(
	logger *slog.Logger,
	insRepo Repository,
	nodeRepo node.Repository,
	chunkRepo chunk.Repository,
) (Service, error) {
	m, err := initMetrics()
	if err != nil {
		return nil, err
	}

	return &svc{
		logger:    logger,
		insRepo:   insRepo,
		nodeRepo:  nodeRepo,
		chunkRepo: chunkRepo,
		metrics:   m,
	}, nil
}

func (s *svc) GetInstance(ctx context.Context, id string) (resource.Instance, error) {
	ins, err := s.insRepo.GetInstanceByID(ctx, id)
	if err != nil {
		return resource.Instance{}, err
	}
	return ins, nil
}

func (s *svc) ListInstances(ctx context.Context, pageSize int, afterID *string) ([]resource.Instance, error) {
	l, err := s.insRepo.ListInstances(ctx, pageSize, afterID)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (s *svc) RunFlavorVersion(
	ctx context.Context,
	flavorVersionID string,
	ownerID string,
	orderedBy string,
) (resource.Instance, error) {
	n, err := s.nodeRepo.BestNode(ctx)
	if err != nil {
		return resource.Instance{}, fmt.Errorf("best node: %w", err)
	}

	flavorID, err := s.chunkRepo.FlavorIDByFlavorVersionID(ctx, flavorVersionID)
	if err != nil {
		return resource.Instance{}, fmt.Errorf("flavor version id: %w", err)
	}

	flavor, err := s.chunkRepo.FlavorByID(ctx, flavorID)
	if err != nil {
		return resource.Instance{}, fmt.Errorf("flavor version: %w", err)
	}

	if flavor.DeletedAt != nil {
		return resource.Instance{}, apierrs.ErrNotFound
	}

	idx := slices.IndexFunc(flavor.Versions, func(version resource.FlavorVersion) bool {
		return version.ID == flavorVersionID
	})
	if idx == -1 {
		return resource.Instance{}, apierrs.ErrNotFound
	}

	instanceID, err := uuid.NewV7()
	if err != nil {
		return resource.Instance{}, fmt.Errorf("instance id: %w", err)
	}

	version := flavor.Versions[idx]

	ins, err := s.insRepo.CreateInstance(ctx, resource.Instance{
		ID:            instanceID.String(),
		FlavorVersion: flavor.Versions[idx],
		State:         resource.InstanceStatePending,
		Owner: resource.User{
			ID: ownerID,
		},
		OrderedBy: orderedBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, n.ID)
	if err != nil {
		return resource.Instance{}, fmt.Errorf("create instance: %w", err)
	}

	s.metrics.instanceCreatedCount.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("flavor_name", flavor.Name),
			attribute.String("flavor_version", version.Version),
			attribute.String("chunk_name", ins.Chunk.Name),
		),
	)

	return ins, nil
}

func (s *svc) DiscoverInstances(ctx context.Context, nodeID string) ([]resource.Instance, error) {
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

func (s *svc) ReceiveInstanceStatusReports(ctx context.Context, reports []resource.InstanceStatusReport) error {
	if err := s.insRepo.ApplyStatusReports(ctx, reports); err != nil {
		return fmt.Errorf("apply status reports: %w", err)
	}
	return nil
}

// TODO: tests
