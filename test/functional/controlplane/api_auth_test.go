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
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	cperrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/user"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestToken(t *testing.T) {
	tests := []struct {
		name     string
		metadata func(user.User, *ecdsa.PrivateKey) metadata.MD
		err      error
	}{
		{
			name: "valid token",
			metadata: func(u user.User, signingKey *ecdsa.PrivateKey) metadata.MD {
				apiKey, err := jwt.NewBuilder().
					IssuedAt(time.Now()).
					Issuer(fixture.APITokenIssuer).
					Audience([]string{fixture.APITokenIssuer}).
					Claim("user_id", u.ID).
					Build()
				require.NoError(t, err)

				signed, err := jwt.Sign(apiKey, jwt.WithKey(jwa.ES256(), signingKey))
				require.NoError(t, err)

				return metadata.Pairs("authorization", string(signed))
			},
		},
		{
			name: "invalid token wrong issuer",
			metadata: func(u user.User, signingKey *ecdsa.PrivateKey) metadata.MD {
				apiKey, err := jwt.NewBuilder().
					IssuedAt(time.Now()).
					Issuer("WRONG ISSUER").
					Audience([]string{fixture.APITokenIssuer}).
					Claim("user_id", u.ID).
					Build()
				require.NoError(t, err)

				signed, err := jwt.Sign(apiKey, jwt.WithKey(jwa.ES256(), signingKey))
				require.NoError(t, err)

				return metadata.Pairs("authorization", string(signed))
			},
			err: cperrs.ErrInvalidToken.GRPCStatus().Err(),
		},
		{
			name: "invalid token wrong audience",
			metadata: func(u user.User, signingKey *ecdsa.PrivateKey) metadata.MD {
				apiKey, err := jwt.NewBuilder().
					IssuedAt(time.Now()).
					Issuer(fixture.APITokenIssuer).
					Audience([]string{"WRONG AUDIENCE"}).
					Claim("user_id", u.ID).
					Build()
				require.NoError(t, err)

				signed, err := jwt.Sign(apiKey, jwt.WithKey(jwa.ES256(), signingKey))
				require.NoError(t, err)

				return metadata.Pairs("authorization", string(signed))
			},
			err: cperrs.ErrInvalidToken.GRPCStatus().Err(),
		},
		{
			name: "invalid token signed with wrong key",
			metadata: func(u user.User, signingKey *ecdsa.PrivateKey) metadata.MD {
				apiKey, err := jwt.NewBuilder().
					IssuedAt(time.Now()).
					Issuer(fixture.APITokenIssuer).
					Audience([]string{fixture.APITokenIssuer}).
					Claim("user_id", u.ID).
					Build()
				require.NoError(t, err)

				key, err := rsa.GenerateKey(rand.Reader, 2048)
				require.NoError(t, err)

				signed, err := jwt.Sign(apiKey, jwt.WithKey(jwa.RS256(), key))
				require.NoError(t, err)

				return metadata.Pairs("authorization", string(signed))
			},
			err: cperrs.ErrInvalidToken.GRPCStatus().Err(),
		},
		{
			name: "invalid token no jwt",
			metadata: func(u user.User, signingKey *ecdsa.PrivateKey) metadata.MD {
				return metadata.Pairs("authorization", "some-non-jwt-string")
			},
			err: cperrs.ErrInvalidToken.GRPCStatus().Err(),
		},
		{
			name: "auth header missing",
			metadata: func(u user.User, signingKey *ecdsa.PrivateKey) metadata.MD {
				return metadata.Pairs("lole", "blabla")
			},
			err: cperrs.ErrAuthHeaderMissing.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
				u   = fixture.User()
				c   = fixture.Chunk()
			)

			cp.Run(t)
			client := cp.ChunkClient(t)

			cp.Postgres.CreateUser(t, &u)

			out := metadata.NewOutgoingContext(ctx, tt.metadata(u, cp.SigningKey))

			_, err := client.CreateChunk(out, &chunkv1alpha1.CreateChunkRequest{
				Name:        c.Name,
				Description: c.Description,
				Tags:        c.Tags,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}
