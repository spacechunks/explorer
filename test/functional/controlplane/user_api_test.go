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
	"github.com/spacechunks/explorer/internal/resource"
	"github.com/spacechunks/explorer/test/fixture"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser(t *testing.T) {
	tests := []struct {
		name                string
		createdUser         *resource.User
		user                resource.User
		acceptPrivacyPolicy bool
		err                 error
	}{
		{
			name:                "user does not exist",
			user:                fixture.User(),
			acceptPrivacyPolicy: true,
		},
		{
			name: "user with nickname already exists",
			createdUser: new(fixture.User(func(tmp *resource.User) {
				tmp.Email = "different@email.com"
			})),
			user:                fixture.User(),
			err:                 apierrs.ErrAlreadyExists.GRPCStatus().Err(),
			acceptPrivacyPolicy: true,
		},
		{
			name: "user with email already exists",
			createdUser: new(fixture.User(func(tmp *resource.User) {
				tmp.Nickname = "different-nickname"
			})),
			user:                fixture.User(),
			err:                 apierrs.ErrAlreadyExists.GRPCStatus().Err(),
			acceptPrivacyPolicy: true,
		},
		{
			name: "does not accept privacy policy",
			createdUser: new(fixture.User(func(tmp *resource.User) {
				tmp.Nickname = "different-nickname"
				tmp.Email = "different@example.com"
			})),
			user: fixture.User(func(tmp *resource.User) {
				tmp.Nickname = "different"
				tmp.Email = "different@example.com"
			}),
			acceptPrivacyPolicy: false,
			err:                 apierrs.ErrPrivacyPolicyNotAccepted.GRPCStatus().Err(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				cp  = fixture.NewControlPlane(t)
			)

			cp.Run(t)

			idTok := cp.IDP.AccessToken(t)

			if tt.createdUser != nil {
				cp.Postgres.CreateUser(t, &tt.user)
			}

			client := cp.UserClient(t)

			_, err := client.Register(ctx, &userv1alpha1.RegisterRequest{
				Nickname:            tt.user.Nickname,
				IdToken:             idTok,
				AcceptPrivacyPolicy: tt.acceptPrivacyPolicy,
			})

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}
