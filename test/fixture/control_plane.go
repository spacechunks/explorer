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

package fixture

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
)

const (
	ControlPlaneAddr = "localhost:9012"
	BaseImage        = "base-image:latest"
	OAuthClientID    = "public-functest-client"
	APITokenIssuer   = "functest-issuer.explorer.chunks.cloud"
)

type ControlPlaneRunOption func(*ControlPlaneRunOptions)

type ControlPlaneRunOptions struct {
	OCIRegistryEndpoint string
	FakeS3Endpoint      string
	OAuthClientID       string
	OAuthIssuerURL      string
}

func WithOCIRegistryEndpoint(endpoint string) ControlPlaneRunOption {
	return func(opts *ControlPlaneRunOptions) {
		opts.OCIRegistryEndpoint = endpoint
	}
}

func WithFakeS3Endpoint(endpoint string) ControlPlaneRunOption {
	return func(opts *ControlPlaneRunOptions) {
		opts.FakeS3Endpoint = endpoint
	}
}

func WithOAuthIssuerEndpoint(issuerURL string) ControlPlaneRunOption {
	return func(opts *ControlPlaneRunOptions) {
		opts.OAuthIssuerURL = issuerURL
	}
}

func RunControlPlane(t *testing.T, pg *Postgres, opts ...ControlPlaneRunOption) {
	ctx := context.Background()
	pg.Run(t, ctx)

	defaultOpts := ControlPlaneRunOptions{
		OCIRegistryEndpoint: "http://localhost:5000",
		FakeS3Endpoint:      "http://localhost:3080",
		OAuthClientID:       OAuthClientID,
	}

	for _, opt := range opts {
		opt(&defaultOpts)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	var keyPem bytes.Buffer
	err = pem.Encode(&keyPem, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	})
	require.NoError(t, err)

	var (
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "control-plane")
		server = controlplane.NewServer(
			logger, controlplane.Config{
				ListenAddr:                    ControlPlaneAddr,
				DBConnString:                  pg.ConnString,
				OCIRegistry:                   defaultOpts.OCIRegistryEndpoint,
				OCIRegistryUser:               OCIRegsitryUser,
				OCIRegistryPass:               OCIRegistryPass,
				BaseImage:                     fmt.Sprintf("%s/base-image:latest", defaultOpts.OCIRegistryEndpoint),
				ImageCacheDir:                 t.TempDir(),
				ImagePlatform:                 "",
				CheckpointJobTimeout:          20 * time.Second,
				CheckpointStatusCheckInterval: 1 * time.Second,
				Bucket:                        Bucket,
				AccessKey:                     "accesskey",
				SecretKey:                     "secretkey",
				// should stay at 2 seconds so TestGetUploadURLRenews passes
				PresignedURLExpiry: 2 * time.Second,
				UsePathStyle:       false,
				OAuthClientID:      defaultOpts.OAuthClientID,
				OAuthIssuerURL:     defaultOpts.OAuthIssuerURL,
				APITokenIssuer:     APITokenIssuer,
				APITokenExpiry:     5 * time.Second,
				APITokenSigningKey: keyPem.String(),
			})
	)

	t.Cleanup(func() {
		server.Stop()
	})

	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	test.WaitServerReady(t, "tcp", ControlPlaneAddr, 20*time.Second)
}
