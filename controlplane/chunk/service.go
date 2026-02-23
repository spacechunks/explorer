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

package chunk

import (
	"context"
	"log/slog"
	"time"

	"github.com/spacechunks/explorer/controlplane/authz"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/resource"
)

type Service interface {
	CreateChunk(ctx context.Context, chunk resource.Chunk) (resource.Chunk, error)
	GetChunk(ctx context.Context, id string) (resource.Chunk, error)
	UpdateChunk(ctx context.Context, new resource.Chunk) (resource.Chunk, error)
	ListChunks(ctx context.Context) ([]resource.Chunk, error)
	CreateFlavor(ctx context.Context, chunkID string, flavor resource.Flavor) (resource.Flavor, error)
	CreateFlavorVersion(
		ctx context.Context,
		flavorID string,
		version resource.FlavorVersion,
	) (resource.FlavorVersion, resource.FlavorVersionDiff, error)
	BuildFlavorVersion(ctx context.Context, versionID string) error
	GetUploadURL(ctx context.Context, flavorVersionID string, tarballHash string) (string, error)
	GetSupportedMinecraftVersions(ctx context.Context) ([]string, error)
	UpdateThumbnail(ctx context.Context, chunkID string, imageData []byte) error
}

type Config struct {
	Registry           string
	BaseImage          string
	Bucket             string
	PresignedURLExpiry time.Duration
	ThumbnailMaxSizeKB int
}

type svc struct {
	logger    *slog.Logger
	repo      Repository
	jobClient job.Client
	s3Store   blob.S3Store
	cfg       Config
	access    authz.AccessEvaluator
}

func NewService(
	logger *slog.Logger,
	repo Repository,
	jobClient job.Client,
	s3Store blob.S3Store,
	access authz.AccessEvaluator,
	cfg Config,
) Service {
	return &svc{
		logger:    logger,
		repo:      repo,
		jobClient: jobClient,
		s3Store:   s3Store,
		access:    access,
		cfg:       cfg,
	}
}
