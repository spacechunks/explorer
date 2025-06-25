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

package controlplane

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	cperrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/image"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Server struct {
	logger *slog.Logger
	cfg    Config
	stopCh chan struct{}
}

func NewServer(logger *slog.Logger, cfg Config) *Server {
	return &Server{
		logger: logger,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

func (s *Server) Run(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, s.cfg.DBConnString)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	var (
		db         = postgres.NewDB(s.logger, pool)
		blobStore  = blob.NewPGStore(db)
		imgService = image.NewService(s.logger, s.cfg.OCIRegistryUser, s.cfg.OCIRegistryPass, s.cfg.ImageCacheDir)
	)

	riverClient, err := CreateRiverClient(
		s.logger,
		db,
		imgService,
		blobStore,
		pool,
		s.cfg.CheckpointJobTimeout,
		s.cfg.CheckpointStatusCheckInterval,
		db,
		db,
	)
	if err != nil {
		return fmt.Errorf("create river client: %w", err)
	}

	db.SetRiverClient(riverClient)

	var (
		grpcServer = grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()),
			// this option is important to set, because
			// flavor file upload will exceed the default
			// size of 4mb.
			grpc.MaxRecvMsgSize(s.cfg.MaxGRPCMessageSize),
			grpc.UnaryInterceptor(errorInterceptor(s.logger)),
		)
		chunkService = chunk.NewService(db, blobStore, db, s.cfg.OCIRegistry, s.cfg.BaseImage)
		chunkServer  = chunk.NewServer(chunkService)
		insService   = instance.NewService(s.logger, db, chunkService)
		insServer    = instance.NewServer(insService)
	)

	instancev1alpha1.RegisterInstanceServiceServer(grpcServer, insServer)
	chunkv1alpha1.RegisterChunkServiceServer(grpcServer, chunkServer)

	if err := riverClient.Start(ctx); err != nil {
		return fmt.Errorf("start river client: %w", err)
	}

	lis, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	g := multierror.Group{}
	g.Go(func() error {
		if err := grpcServer.Serve(lis); err != nil {
			s.Stop()
			return fmt.Errorf("failed to serve mgmt server: %w", err)
		}
		return nil
	})

	<-s.stopCh

	// add stop-related code below

	grpcServer.GracefulStop()

	if err := riverClient.Stop(ctx); err != nil {
		s.logger.ErrorContext(ctx, "failed to stop river client", "err", err)
	}

	return g.Wait().ErrorOrNil()
}

func (s *Server) Stop() {
	s.stopCh <- struct{}{}
}

func errorInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			var e cperrs.Error
			if errors.As(err, &e) {
				return nil, e.GRPCStatus().Err()
			}

			logger.ErrorContext(
				ctx,
				"internal service error occurred",
				"method", info.FullMethod,
				"err", err,
			)

			return nil, status.Error(codes.Internal, "internal service error occurred")
		}
		return resp, nil
	}
}

func CreateRiverClient(
	logger *slog.Logger,
	repo chunk.Repository,
	imgService image.Service,
	blobStore blob.Store,
	pool *pgxpool.Pool,
	checkpointTimeout time.Duration,
	statusCheckInterval time.Duration,
	nodeRepo node.Repository,
	jobClient job.Client,
) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()

	imgWorker := worker.NewCreateImageWorker(repo, blobStore, imgService, jobClient)
	if err := river.AddWorkerSafely[job.CreateImage](workers, imgWorker); err != nil {
		return nil, fmt.Errorf("add create image worker: %w", err)
	}

	checkWorker := worker.NewCheckpointWorker(
		logger.With("component", "checkpoint-worker"),
		createCheckpointClient,
		checkpointTimeout,
		statusCheckInterval,
		nodeRepo,
	)
	if err := river.AddWorkerSafely[job.CreateCheckpoint](workers, checkWorker); err != nil {
		return nil, fmt.Errorf("add create checkpoint worker: %w", err)
	}

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 10,
			},
		},
		Workers:     workers,
		Logger:      logger.With("component", "river"),
		MaxAttempts: 5,
	})
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}

	return riverClient, nil
}

func createCheckpointClient(host string) (checkpointv1alpha1.CheckpointServiceClient, error) {
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return checkpointv1alpha1.NewCheckpointServiceClient(conn), nil
}
