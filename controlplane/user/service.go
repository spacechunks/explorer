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
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/spacechunks/explorer/controlplane/resource"
)

type Service interface {
	Register(ctx context.Context, nickname string, rawIDToken string) error
	Login(ctx context.Context, rawIDToken string) (resource.User, []byte, error)
}

type service struct {
	repo           Repository
	provider       *oidc.Provider
	clientID       string
	issuer         string
	apiTokenExpiry time.Duration
	signingKey     *ecdsa.PrivateKey
}

type idTokenClaims struct {
	Email string `json:"email"`
}

func NewService(
	repo Repository,
	provider *oidc.Provider,
	oauthClientID string,
	issuer string,
	apiTokenExpiry time.Duration,
	signingKey *ecdsa.PrivateKey,
) Service {
	return &service{
		repo:           repo,
		provider:       provider,
		clientID:       oauthClientID,
		issuer:         issuer,
		apiTokenExpiry: apiTokenExpiry,
		signingKey:     signingKey,
	}
}

func (s *service) Register(ctx context.Context, nickname string, rawIDToken string) error {
	verifier := s.provider.Verifier(&oidc.Config{
		ClientID: s.clientID,
	})

	idTok, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return fmt.Errorf("verify token: %w", err)
	}

	var claims idTokenClaims
	if err := idTok.Claims(&claims); err != nil {
		return fmt.Errorf("parse token claims: %w", err)
	}

	if _, err := s.repo.CreateUser(ctx, resource.User{
		Nickname: nickname,
		Email:    claims.Email,
	}); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (s *service) Login(ctx context.Context, rawIDToken string) (resource.User, []byte, error) {
	verifier := s.provider.Verifier(&oidc.Config{
		ClientID: s.clientID,
	})

	idTok, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return resource.User{}, nil, fmt.Errorf("verify token: %w", err)
	}

	var claims idTokenClaims
	if err := idTok.Claims(&claims); err != nil {
		return resource.User{}, nil, fmt.Errorf("parse token claims: %w", err)
	}

	u, err := s.repo.GetUserByEmail(ctx, claims.Email)
	if err != nil {
		return resource.User{}, nil, fmt.Errorf("get user: %w", err)
	}

	iss := time.Now()
	apiTok, err := jwt.NewBuilder().
		IssuedAt(iss).
		Issuer(s.issuer).
		Audience([]string{s.issuer}).
		Expiration(iss.Add(s.apiTokenExpiry)).
		Claim("user_id", u.ID).
		Claim("email", claims.Email).
		Build()
	if err != nil {
		return resource.User{}, nil, fmt.Errorf("create token: %w", err)
	}

	signed, err := jwt.Sign(apiTok, jwt.WithKey(jwa.ES256(), s.signingKey))
	if err != nil {
		return resource.User{}, nil, fmt.Errorf("sign token: %w", err)
	}

	return u, signed, nil
}
