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

	"github.com/google/go-cmp/cmp"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/resource"
	"github.com/spacechunks/explorer/internal/ptr"
	"github.com/spacechunks/explorer/test"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRegisterUser(t *testing.T) {
	tests := []struct {
		name        string
		createdUser *resource.User
		user        resource.User
		err         error
	}{
		{
			name: "user does not exist",
			user: fixture.User(),
		},
		{
			name: "user with nickname already exists",
			createdUser: ptr.Pointer(fixture.User(func(tmp *resource.User) {
				tmp.Email = "different@email.com"
			})),
			user: fixture.User(),
			err:  apierrs.ErrAlreadyExists.GRPCStatus().Err(),
		},
		{
			name: "user with email already exists",
			createdUser: ptr.Pointer(fixture.User(func(tmp *resource.User) {
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
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)

			idTok := cp.IDP.IDToken(t)

			if tt.createdUser != nil {
				cp.Postgres.CreateUser(t, &tt.user)
			}

			client := cp.UserClient(t)

			_, err := client.Register(ctx, &userv1alpha1.RegisterRequest{
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

func TestLoginUser(t *testing.T) {
	tests := []struct {
		name       string
		user       resource.User
		createUser bool
		err        error
	}{
		{
			name:       "user can login",
			user:       fixture.User(),
			createUser: true,
		},
		{
			name: "user doesnt exist",
			user: fixture.User(),
			err:  apierrs.ErrNotFound.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)

			idTok := cp.IDP.IDToken(t)

			if tt.createUser {
				cp.Postgres.CreateUser(t, &tt.user)
			}

			client := cp.UserClient(t)

			resp, err := client.Login(ctx, &userv1alpha1.LoginRequest{
				IdToken: idTok,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)

			expected := &userv1alpha1.User{
				Id:        tt.user.ID,
				Nickname:  tt.user.Nickname,
				CreatedAt: timestamppb.New(tt.user.CreatedAt),
				UpdatedAt: timestamppb.New(tt.user.UpdatedAt),
			}

			if d := cmp.Diff(expected, resp.User, protocmp.Transform(), test.IgnoredProtoUserFields); d != "" {
				t.Fatalf("mismatch (-want +got):\n%s", d)
			}
		})
	}
}
