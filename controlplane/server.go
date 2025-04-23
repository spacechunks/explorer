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

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgxpool"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	cperrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Server struct {
	logger *slog.Logger
	cfg    Config
}

func NewServer(logger *slog.Logger, cfg Config) *Server {
	return &Server{
		logger: logger,
		cfg:    cfg,
	}
}

func (s *Server) Run(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, s.cfg.DBConnString)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	var (
		db         = postgres.NewDB(s.logger, pool)
		grpcServer = grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()),
			// this option is important to set, because
			// flavor file upload will exceed the default
			// size of 4mb.
			grpc.MaxRecvMsgSize(s.cfg.MaxGRPCMessageSize),
			grpc.UnaryInterceptor(errorInterceptor),
		)
		blobStore    = blob.NewPGStore(db)
		chunkService = chunk.NewService(db, blobStore)
		chunkServer  = chunk.NewServer(chunkService)
		insService   = instance.NewService(s.logger, db, chunkService)
		insServer    = instance.NewServer(insService)
	)

	instancev1alpha1.RegisterInstanceServiceServer(grpcServer, insServer)
	chunkv1alpha1.RegisterChunkServiceServer(grpcServer, chunkServer)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lis, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	g := multierror.Group{}
	g.Go(func() error {
		if err := grpcServer.Serve(lis); err != nil {
			cancel()
			return fmt.Errorf("failed to serve mgmt server: %w", err)
		}
		return nil
	})

	<-ctx.Done()

	// add stop related code below

	grpcServer.GracefulStop()

	return g.Wait().ErrorOrNil()
}

func errorInterceptor(
	ctx context.Context,
	req any,
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		var e cperrs.Error
		if errors.As(err, &e) {
			return nil, e.GRPCStatus().Err()
		}
		return nil, status.Error(codes.Internal, "internal service error occured")
	}
	return resp, nil
}
