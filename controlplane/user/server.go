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

package user

import (
	"context"

	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	userv1alpha1.UnimplementedUserServiceServer
	service Service
}

func NewServer(service Service) *Server {
	return &Server{
		service: service,
	}
}

func (s Server) Register(
	ctx context.Context,
	req *userv1alpha1.RegisterRequest,
) (*userv1alpha1.RegisterResponse, error) {
	if err := s.service.Register(ctx, req.Nickname, req.IdToken); err != nil {
		return nil, err
	}
	return &userv1alpha1.RegisterResponse{}, nil
}

func (s Server) Login(ctx context.Context, req *userv1alpha1.LoginRequest) (*userv1alpha1.LoginResponse, error) {
	user, apiKey, err := s.service.Login(ctx, req.IdToken)
	if err != nil {
		return nil, err
	}
	return &userv1alpha1.LoginResponse{
		User: &userv1alpha1.User{
			Id:        user.ID,
			Nickname:  user.Nickname,
			CreatedAt: timestamppb.New(user.CreatedAt),
			UpdatedAt: timestamppb.New(user.UpdatedAt),
		},
		ApiToken: string(apiKey),
	}, nil
}
