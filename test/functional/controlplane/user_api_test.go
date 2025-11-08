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
	"testing"

	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/user"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRegisterUser(t *testing.T) {
	tests := []struct {
		name        string
		createdUser *user.User
		user        user.User
		err         error
	}{
		{
			name: "user does not exist",
			user: fixture.User(),
		},
		{
			name: "user with nickname already exists",
			createdUser: ptr.Pointer(fixture.User(func(tmp *user.User) {
				tmp.Email = "different@email.com"
			})),
			user: fixture.User(),
			err:  apierrs.ErrAlreadyExists.GRPCStatus().Err(),
		},
		{
			name: "user with email already exists",
			createdUser: ptr.Pointer(fixture.User(func(tmp *user.User) {
				tmp.Nickname = "different-nickname"
			})),
			user: fixture.User(),
			err:  apierrs.ErrAlreadyExists.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				pg  = fixture.NewPostgres()
			)

			idp := fixture.RunIDP(t)

			fixture.RunControlPlane(t, pg, fixture.WithOAuthIssuerEndpoint(idp.Endpoint))

			idTok := idp.IDToken(t)

			if tt.createdUser != nil {
				pg.CreateUser(t, &tt.user)
			}

			conn, err := grpc.NewClient(
				fixture.ControlPlaneAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			client := userv1alpha1.NewUserServiceClient(conn)

			_, err = client.Register(ctx, &userv1alpha1.RegisterRequest{
				Nickname: tt.user.Nickname,
				IdToken:  idTok,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}

}
