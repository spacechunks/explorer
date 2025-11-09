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

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/controlplane"
	"github.com/spacechunks/explorer/controlplane/user"
	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	ControlPlaneAddr = "localhost:9012"
	BaseImage        = "base-image:latest"
	OAuthClientID    = "public-functest-client"
	APITokenIssuer   = "functest-issuer.explorer.chunks.cloud"
)

type ControlPlane struct {
	Postgres   *Postgres
	IDP        *IDP
	SigningKey *ecdsa.PrivateKey
}

func NewControlPlane(t *testing.T) ControlPlane {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return ControlPlane{
		Postgres:   NewPostgres(),
		IDP:        NewIDP(),
		SigningKey: key,
	}
}

type ControlPlaneRunOption func(*ControlPlaneRunOptions)

type ControlPlaneRunOptions struct {
	OCIRegistryEndpoint string
	FakeS3Endpoint      string
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

func (c ControlPlane) Run(t *testing.T, opts ...ControlPlaneRunOption) {
	ctx := context.Background()

	c.Postgres.Run(t, ctx)
	c.IDP.Run(t)

	defaultOpts := ControlPlaneRunOptions{
		OCIRegistryEndpoint: "http://localhost:5000",
		FakeS3Endpoint:      "http://localhost:3080",
	}

	for _, opt := range opts {
		opt(&defaultOpts)
	}

	der, err := x509.MarshalECPrivateKey(c.SigningKey)
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
				DBConnString:                  c.Postgres.ConnString,
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
				OAuthClientID:      OAuthClientID,
				OAuthIssuerURL:     c.IDP.Endpoint,
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

// AddUserAPIKey generates a new signed api token for the given user id
// and creates a grpc metadata pair that will be added to the passed context.
func (c ControlPlane) AddUserAPIKey(t *testing.T, ctx *context.Context, u user.User) {
	apiKey, err := jwt.NewBuilder().
		IssuedAt(time.Now()).
		Issuer(APITokenIssuer).
		Audience([]string{APITokenIssuer}).
		Claim("user_id", u.ID).
		Build()
	require.NoError(t, err)

	signed, err := jwt.Sign(apiKey, jwt.WithKey(jwa.ES256(), c.SigningKey))
	require.NoError(t, err)

	md := metadata.Pairs("authorization", string(signed))
	out := metadata.NewOutgoingContext(*ctx, md)
	*ctx = out
}

func (c ControlPlane) ChunkClient(t *testing.T) chunkv1alpha1.ChunkServiceClient {
	conn, err := grpc.NewClient(
		ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return chunkv1alpha1.NewChunkServiceClient(conn)
}

func (c ControlPlane) InstanceClient(t *testing.T) instancev1alpha1.InstanceServiceClient {
	conn, err := grpc.NewClient(
		ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return instancev1alpha1.NewInstanceServiceClient(conn)
}

func (c ControlPlane) UserClient(t *testing.T) userv1alpha1.UserServiceClient {
	conn, err := grpc.NewClient(
		ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return userv1alpha1.NewUserServiceClient(conn)
}
