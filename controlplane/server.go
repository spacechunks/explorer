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
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	checkpointv1alpha1 "github.com/spacechunks/explorer/api/platformd/checkpoint/v1alpha1"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/controlplane/blob"
	"github.com/spacechunks/explorer/controlplane/chunk"
	"github.com/spacechunks/explorer/controlplane/contextkeys"
	cperrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/instance"
	"github.com/spacechunks/explorer/controlplane/job"
	"github.com/spacechunks/explorer/controlplane/node"
	"github.com/spacechunks/explorer/controlplane/postgres"
	"github.com/spacechunks/explorer/controlplane/user"
	"github.com/spacechunks/explorer/controlplane/worker"
	"github.com/spacechunks/explorer/internal/image"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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
	oidcProvider, err := oidc.NewProvider(ctx, s.cfg.OAuthIssuerURL)
	if err != nil {
		return fmt.Errorf("oidc provider: %w", err)
	}

	pool, err := pgxpool.New(ctx, s.cfg.DBConnString)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	s3cfg, err := awscfg.LoadDefaultConfig(
		ctx,
		awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s.cfg.AccessKey, s.cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}

	var (
		s3client = s3.NewFromConfig(s3cfg, func(o *s3.Options) {
			o.UsePathStyle = s.cfg.UsePathStyle
		})
		db         = postgres.NewDB(s.logger, pool)
		blobStore  = blob.NewS3Store(s.cfg.Bucket, s3client, s3.NewPresignClient(s3client))
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
		s.cfg.ImagePlatform,
	)
	if err != nil {
		return fmt.Errorf("create river client: %w", err)
	}

	db.SetRiverClient(riverClient)

	pemBlock, _ := pem.Decode([]byte(s.cfg.APITokenSigningKey))

	key, err := x509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse ec private key: %w", err)
	}

	var (
		grpcServer = grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()),
			grpc.ChainUnaryInterceptor(errorInterceptor(s.logger), authInterceptor(key)),
		)

		userService = user.NewService(
			db,
			oidcProvider,
			s.cfg.OAuthClientID,
			s.cfg.APITokenIssuer,
			s.cfg.APITokenExpiry,
			key,
		)
		userServer   = user.NewServer(userService)
		chunkService = chunk.NewService(
			db,
			db,
			blobStore,
			chunk.Config{
				Registry:           s.cfg.OCIRegistry,
				BaseImage:          s.cfg.BaseImage,
				Bucket:             s.cfg.Bucket,
				PresignedURLExpiry: s.cfg.PresignedURLExpiry,
			})
		chunkServer = chunk.NewServer(chunkService)
		insService  = instance.NewService(s.logger, db, db, chunkService)
		insServer   = instance.NewServer(insService)
	)

	instancev1alpha1.RegisterInstanceServiceServer(grpcServer, insServer)
	chunkv1alpha1.RegisterChunkServiceServer(grpcServer, chunkServer)
	userv1alpha1.RegisterUserServiceServer(grpcServer, userServer)

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

func authInterceptor(signingKey *ecdsa.PrivateKey) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// we only need to check if it's the user server and login call.
		// doing it like this, will not break anything if we change the
		// version of the user api.
		if strings.HasSuffix(info.FullMethod, "UserService/Register") ||
			strings.HasSuffix(info.FullMethod, "UserService/Login") {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "missing metadata")
		}

		vals := md.Get("authorization")
		if len(vals) == 0 {
			return nil, cperrs.ErrAuthHeaderMissing
		}

		tok, err := jwt.Parse([]byte(vals[0]), jwt.WithKey(jwa.ES256(), signingKey))
		if err != nil {
			return nil, cperrs.ErrInvalidToken
		}

		ctx = context.WithValue(ctx, contextkeys.APIToken, tok)

		return handler(ctx, req)
	}
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

type fixedRetryPolicy struct {
	delay time.Duration
}

func (p *fixedRetryPolicy) NextRetry(_ *rivertype.JobRow) time.Time {
	return time.Now().Add(p.delay)
}

func CreateRiverClient(
	logger *slog.Logger,
	chunkRepo chunk.Repository,
	imgService image.Service,
	blobStore blob.S3Store,
	pool *pgxpool.Pool,
	checkpointTimeout time.Duration,
	statusCheckInterval time.Duration,
	nodeRepo node.Repository,
	jobClient job.Client,
	imagePlatform string,
) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()

	imgWorker := worker.NewCreateImageWorker(
		logger.With("component", "image-worker"),
		chunkRepo,
		imgService,
		jobClient,
		blobStore,
		imagePlatform,
	)
	if err := river.AddWorkerSafely[job.CreateImage](workers, imgWorker); err != nil {
		return nil, fmt.Errorf("add create image worker: %w", err)
	}

	checkWorker := worker.NewCheckpointWorker(
		logger.With("component", "checkpoint-worker"),
		createCheckpointClient,
		checkpointTimeout,
		statusCheckInterval,
		nodeRepo,
		chunkRepo,
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
		MaxAttempts: 5, // TODO: configurable
		RetryPolicy: &fixedRetryPolicy{
			delay: time.Second * 5, // TODO: configurable
		},
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
