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

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/peterbourgon/ff/v3"
	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/controlplane/postgres/migrations"
)

func main() {
	var (
		logger                   = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		fs                       = flag.NewFlagSet("controlplane", flag.ContinueOnError)
		listenAddr               = fs.String("listen-address", ":9012", "address and port the control plane server listens on")                                                                                   //nolint:lll
		pgConnString             = fs.String("postgres-dsn", "", "connection string in the form of postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]")                                    //nolint:lll
		ociRegistry              = fs.String("oci-registry", "", "registry to use to pull and push images")                                                                                                       //nolint:lll
		ociRegistryUser          = fs.String("oci-registry-user", "", "oci registry username used for authentication against configured oci registry")                                                            //nolint:lll
		ociRegistryPass          = fs.String("oci-registry-pass", "", "oci registry password used for authentication against configured oci registry")                                                            //nolint:lll
		baseImage                = fs.String("base-image", "", "base image to use for creating flavor version images")                                                                                            //nolint:lll
		imageCacheDir            = fs.String("image-cache-dir", "/tmp/explorer-images", "directory used to cache base image")                                                                                     //nolint:lll
		imagePlatform            = fs.String("image-platform", "linux/amd64", "the platform that will be specified when pulling the base image. must match with all configured platformd hosts e.g. linux/amd64") //nolint:lll
		checkJobTimeout          = fs.Duration("checkpoint-job-timeout", 5*time.Minute, "when to abort the checkpointing job")                                                                                    //nolint:lll
		checkStatusCheckInterval = fs.Duration("checkpoint-status-check-interval", 3*time.Second, "how often the status check endpoint for a checkpoint should be called")                                        //nolint:lll
		bucket                   = fs.String("bucket", "explorer-data", "bucket to use for storing change sets and backend for content-addressable storage")                                                      //nolint:lll
		accessKey                = fs.String("access-key", "", "access key to use for accessing the bucket")                                                                                                      //nolint:lll
		secretKey                = fs.String("secret-key", "", "secret key to use for accessing the bucket")                                                                                                      //nolint:lll
		presignedURLExpiry       = fs.Duration("presigned-url-expiry", 5*time.Minute, "when to expire the presigned URL")                                                                                         //nolint:lll
		usePathStyle             = fs.Bool("use-path-style", true, "whether to use path style to access the bucket")                                                                                              //nolint:lll
		idpOAuthClientID         = fs.String("oauth-client-id", "", "oauth client ID to use for authentication")
		idpOAuthIssuerEndpoint   = fs.String("idp-oauth-issuer-endpoint", "", "issuer endpoint to use for authentication")
		apiTokenIssuer           = fs.String("api-token-issuer", "", "issuer to use for api tokens issued by the control plane")
		apiTokenExpiry           = fs.Duration("api-token-expiry", 10*time.Minute, "expiry of api tokens issued by the control plane")
		apiTokenSigningKey       = fs.String("api-token-signing-key", "", "key used to sign api tokens issued by the control plane")
	)
	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("CONTROLPLANE"),
	); err != nil {
		die(logger, "failed to parse config", err)
	}

	var (
		cfg = controlplane.Config{
			ListenAddr:                    *listenAddr,
			DBConnString:                  *pgConnString,
			OCIRegistry:                   *ociRegistry,
			OCIRegistryUser:               *ociRegistryUser,
			OCIRegistryPass:               *ociRegistryPass,
			BaseImage:                     *baseImage,
			ImageCacheDir:                 *imageCacheDir,
			ImagePlatform:                 *imagePlatform,
			CheckpointJobTimeout:          *checkJobTimeout,
			CheckpointStatusCheckInterval: *checkStatusCheckInterval,
			Bucket:                        *bucket,
			AccessKey:                     *accessKey,
			SecretKey:                     *secretKey,
			PresignedURLExpiry:            *presignedURLExpiry,
			UsePathStyle:                  *usePathStyle,
			OAuthClientID:                 *idpOAuthClientID,
			OAuthIssuerURL:                *idpOAuthIssuerEndpoint,
			APITokenIssuer:                *apiTokenIssuer,
			APITokenExpiry:                *apiTokenExpiry,
			APITokenSigningKey:            *apiTokenSigningKey,
		}
		ctx    = context.Background()
		server = controlplane.NewServer(logger, cfg)
	)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		s := <-c
		logger.Info("received shutdown signal", "signal", s)
		server.Stop()
	}()

	if err := migrations.Migrate(cfg.DBConnString); err != nil {
		die(logger, "failed to run migrations", err)
	}

	if err := server.Run(ctx); err != nil {
		var multi *multierror.Error
		if errors.As(err, &multi) {
			errs := make([]string, 0, len(multi.WrappedErrors()))
			for _, err := range multi.WrappedErrors() {
				errs = append(errs, err.Error())
			}
			die(logger, "failed to run server", errors.New(strings.Join(errs, ",")))
			return
		}
		die(logger, "failed to run server", err)
	}
}

func die(logger *slog.Logger, msg string, err error) {
	logger.Error(msg, "err", err)
	os.Exit(1)
}
