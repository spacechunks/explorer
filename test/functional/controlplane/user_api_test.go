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
			user:                fixture.IDPUser,
			acceptPrivacyPolicy: true,
		},
		{
			name:                "user with nickname already exists",
			createdUser:         new(fixture.IDPUser),
			user:                fixture.IDPUser,
			err:                 apierrs.ErrAlreadyExists.GRPCStatus().Err(),
			acceptPrivacyPolicy: true,
		},
		{
			name:                "user with same idp id already exists",
			createdUser:         new(fixture.IDPUser),
			user:                fixture.IDPUser,
			err:                 apierrs.ErrAlreadyExists.GRPCStatus().Err(),
			acceptPrivacyPolicy: true,
		},
		{
			name:                "does not accept privacy policy",
			createdUser:         new(fixture.OtherIDPUser),
			user:                fixture.OtherIDPUser,
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

			if tt.createdUser != nil {
				cp.Postgres.CreateUser(t, &tt.user)
			}

			cp.AddUserAPIKey(t, &ctx, tt.user)

			client := cp.UserClient(t)

			_, err := client.Register(ctx, &userv1alpha1.RegisterRequest{
				Nickname:            tt.user.Nickname,
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
