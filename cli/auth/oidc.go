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

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/lestrrat-go/jwx/v4/jwt"
	"github.com/pkg/browser"
	"github.com/skip2/go-qrcode"
	"github.com/spacechunks/explorer/cli/state"
	"golang.org/x/oauth2"
)

type Service interface {
	AccessToken(ctx context.Context) (string, error)
}

func NewOIDC(
	logger *slog.Logger,
	state *state.Data,
	clientID string,
	issuerEndpoint string,
	scopes []string,
) *OIDC {
	return &OIDC{
		logger:         logger,
		issuerEndpoint: issuerEndpoint,
		clientID:       clientID,
		state:          state,
		scopes:         scopes,
	}
}

type OIDC struct {
	logger         *slog.Logger
	issuerEndpoint string
	clientID       string
	state          *state.Data
	scopes         []string
}

func (svc OIDC) AccessToken(ctx context.Context) (string, error) {
	if err := svc.validateToken(svc.state.AccessToken); err != nil {
		tok, err := svc.getAccessToken(ctx, svc.scopes)
		if err != nil {
			return "", fmt.Errorf("unable to get access token: %w", err)
		}

		svc.state.Update(state.Data{
			AccessToken: tok,
		})
	}

	// we got what we need: a still valid or newly issued api token.
	// so, we can return. we don't need to check if the id token is
	// valid, because the only thing we need to the control plane
	// is the api token. once it's expired we'll check the id token
	// again and possibly renew it.
	return svc.state.AccessToken, nil
}

type expireEarlier struct {
	dur time.Duration
}

func (c expireEarlier) Now() time.Time {
	return time.Now().Add(c.dur)
}

func (svc OIDC) validateToken(token string) error {
	// we don't only need to return the errors, in order to know
	// that parsing or validation went wrong. we are not really
	// interested in propagating the error up, so we just return
	// err without giving extra context using fmt.Errorf like how
	// it's usually done in this codebase.

	tok, err := jwt.ParseString(token, jwt.WithVerify(false))
	if err != nil {
		svc.logger.Debug("error parsing jwt", "err", err)
		return err
	}

	// we want to expire the token a bit earlier to avoid the edge
	// case where the token is still valid on the users machine, but
	// while sending it to the control plane it expires.
	c := &expireEarlier{
		dur: 5 * time.Minute,
	}

	if err := jwt.Validate(tok, jwt.WithClock(c)); err != nil {
		svc.logger.Debug("error validating jwt", "err", err)
		return err
	}

	return nil
}

func (svc OIDC) getAccessToken(ctx context.Context, scopes []string) (string, error) {
	provider, err := oidc.NewProvider(ctx, svc.issuerEndpoint)
	if err != nil {
		return "", fmt.Errorf("provider: %w", err)
	}

	var (
		cfg = oauth2.Config{
			ClientID: svc.clientID,
			Endpoint: provider.Endpoint(),
			Scopes:   scopes,
		}
	)

	resp, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return "", fmt.Errorf("device auth: %w", err)
	}

	svc.logger.Debug(
		"device auth data",
		"interval", resp.Interval,
		"verification_uri", resp.VerificationURI,
		"verification_uri_complete", resp.VerificationURIComplete,
		"user_code", resp.UserCode,
		"device_code", resp.DeviceCode,
		"expiry", resp.Expiry,
	)

	fmt.Println("Authentication requried:")

	var url string
	if resp.VerificationURIComplete == "" {
		fmt.Printf("Visit: %s\n", resp.VerificationURI)
		fmt.Printf("Enter code: %s\n", resp.UserCode)
		url = resp.VerificationURI
	} else {
		fmt.Printf("Visit: %s\n", resp.VerificationURIComplete)
		url = resp.VerificationURIComplete
	}

	if err := browser.OpenURL(url); err != nil {
		svc.logger.Warn("error opening browser, printing QR code instead", "err", err)
		qr, err := qrcode.New(url, qrcode.Medium)
		if err != nil {
			return "", fmt.Errorf("qrcode: %w", err)
		}
		fmt.Println(qr.ToSmallString(true))
	}

	tok, err := cfg.DeviceAccessToken(ctx, resp)
	if err != nil {
		return "", fmt.Errorf("device token: %w", err)
	}

	return tok.AccessToken, nil
}
