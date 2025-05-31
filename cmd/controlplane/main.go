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

	"github.com/hashicorp/go-multierror"
	"github.com/peterbourgon/ff/v3"
	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/controlplane/postgres/migrations"
)

func main() {
	var (
		logger             = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		fs                 = flag.NewFlagSet("controlplane", flag.ContinueOnError)
		listenAddr         = fs.String("listen-address", ":9012", "address and port the control plane server listens on")                                                //nolint:lll
		pgConnString       = fs.String("postgres-dsn", "", "connection string in the form of postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]") //nolint:lll
		grpcMaxMessageSize = fs.Uint("grpc-max-message-size", 4000000, "maximum grpc message size in bytes")
		ociRegistry        = fs.String("oci-registry", "", "registry to use to pull and push images")
		ociRegistryUser    = fs.String("oci-registry-user", "", "oci registry username used for authentication against configured oci registry")
		ociRegistryPass    = fs.String("oci-registry-pass", "", "oci registry password used for authentication against configured oci registry")
		baseImage          = fs.String("base-image", "", "base image to use for creating flavor version images")
		imageCacheDir      = fs.String("image-cache-dir", "/tmp/explorer-images", "directory used to cache base image")
	)
	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("CONTROLPLANE"),
	); err != nil {
		die(logger, "failed to parse config", err)
	}

	var (
		cfg = controlplane.Config{
			ListenAddr:         *listenAddr,
			DBConnString:       *pgConnString,
			MaxGRPCMessageSize: int(*grpcMaxMessageSize),
			OCIRegistry:        *ociRegistry,
			OCIRegistryUser:    *ociRegistryUser,
			OCIRegistryPass:    *ociRegistryPass,
			BaseImage:          *baseImage,
			ImageCacheDir:      *imageCacheDir,
		}
		ctx, cancel = context.WithCancel(context.Background())
		server      = controlplane.NewServer(logger, cfg)
	)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		s := <-c
		logger.Info("received shutdown signal", "signal", s)
		cancel()
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
