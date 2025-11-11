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
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/auth"
	clicmd "github.com/spacechunks/explorer/cli/cmd"
	"github.com/spacechunks/explorer/cli/fshelper"
	"github.com/spacechunks/explorer/cli/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	ctx := context.Background()

	cfg, err := createOrReadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	conn, err := grpc.NewClient(cfg.ControlPlaneEndpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true,
	})))
	if err != nil {
		die("Failed to create gRPC client", err)
	}

	stateData, err := state.New()
	if err != nil {
		die("Failed to read state data", err)
	}

	userClient := userv1alpha1.NewUserServiceClient(conn)

	oidcAuth, err := auth.NewOIDC(ctx, &stateData, cfg.IDPClientID, cfg.IDPIssuerEndpoint, userClient)
	if err != nil {
		die("Failed to create Microsoft auth service", err)
	}

	var (
		cliContext = cli.Context{
			Config:         cfg,
			Client:         chunkv1alpha1.NewChunkServiceClient(conn),
			InstanceClient: instancev1alpha1.NewInstanceServiceClient(conn),
			UserClient:     userClient,
			Auth:           oidcAuth,
			State:          stateData,
		}
	)

	if err := clicmd.Root(ctx, cliContext).Execute(); err != nil {
		os.Exit(1)
	}
}

func die(msg string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}

func createOrReadConfig() (state.Config, error) {
	cfgHome, err := fshelper.ConfigHome()
	if err != nil {
		return state.Config{}, fmt.Errorf("determine home directory: %w", err)
	}

	cfgPath := filepath.Join(cfgHome, "config.yaml")

	cfg, err := cli.ReadYAMLFile[state.Config](cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := cli.WriteYAMLFile(state.DefaultConfig, cfgPath); err != nil {
				return state.Config{}, fmt.Errorf("write default config: %w", err)
			}
			cfg = state.DefaultConfig
		} else {
			return state.Config{}, fmt.Errorf("read config: %w", err)
		}
	}
	return cfg, nil
}
