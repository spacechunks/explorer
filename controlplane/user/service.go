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
	"fmt"

	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/internal/resource"
)

type Service interface {
	Register(ctx context.Context, nickname string, acceptPrivacyPolicy bool, idpID string) error
}

type service struct {
	repo    Repository
	metrics metrics
}

func NewService(
	repo Repository,
) (Service, error) {
	m, err := initMetrics()
	if err != nil {
		return nil, err
	}

	return &service{
		repo:    repo,
		metrics: m,
	}, nil
}

func (s *service) Register(ctx context.Context, nickname string, acceptPrivacyPolicy bool, idpID string) error {
	if !acceptPrivacyPolicy {
		return apierrs.ErrPrivacyPolicyNotAccepted
	}

	if _, err := s.repo.CreateUser(ctx, resource.User{
		Nickname: nickname,
		IDPID:    idpID,
	}); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	s.metrics.registeredCount.Add(ctx, 1)
	return nil
}
