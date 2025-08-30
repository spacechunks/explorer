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
	"runtime"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	clicmd "github.com/spacechunks/explorer/cli/cmd"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
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

	var (
		ctx   = context.Background()
		state = cli.State{
			Config:         cfg,
			Client:         chunkv1alpha1.NewChunkServiceClient(conn),
			InstanceClient: instancev1alpha1.NewInstanceServiceClient(conn),
		}
	)

	if err := clicmd.Root(ctx, state).Execute(); err != nil {
		os.Exit(1)
	}
}

func die(msg string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}

func createOrReadConfig() (cli.Config, error) {
	cfgHome, err := configHome()
	if err != nil {
		return cli.Config{}, fmt.Errorf("determine home directory: %w", err)
	}

	cfgPath := cfgHome + "/config.yaml"

	cfg, err := cli.ReadYAMLFile[cli.Config](cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := cli.WriteYAMLFile(cli.DefaultConfig, cfgPath); err != nil {
				return cli.Config{}, fmt.Errorf("write default config: %w", err)
			}
			cfg = cli.DefaultConfig
		} else {
			return cli.Config{}, fmt.Errorf("read config: %w", err)
		}
	}
	return cfg, nil
}

func configHome() (string, error) {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome != "" {
		return cfgHome, nil
	}

	switch runtime.GOOS {
	case "windows":
		// %LOCALAPPDATA% should always be present, so no need to create it
		return filepath.Join("%LOCALAPPDATA%", "explorer"), nil
	case "linux", "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get users home directory: %w", err)
		}
		cfgHome = filepath.Join(homeDir, ".config", "explorer")
	}

	if _, err := os.Stat(cfgHome); err == nil {
		return cfgHome, nil
	}

	if err := os.MkdirAll(cfgHome, 0700); err != nil {
		return "", fmt.Errorf("failed to create config home directory %s: %w", cfgHome, err)
	}

	return cfgHome, nil
}
