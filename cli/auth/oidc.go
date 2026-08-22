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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/lestrrat-go/jwx/v4/jwt"
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
			ClientID:    svc.clientID,
			RedirectURL: "http://localhost:64554",
			Endpoint:    provider.Endpoint(),
			Scopes:      scopes,
		}
		//verifier   = oauth2.GenerateVerifier()
		//stateParam = oauth2.GenerateVerifier()
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
	if resp.VerificationURIComplete == "" {
		fmt.Printf("Visit: %s\n", resp.VerificationURI)
		fmt.Printf("Enter code: %d\n", resp.UserCode)
	} else {
		fmt.Printf("Visit: %s\n", resp.VerificationURIComplete)
	}

	tok, err := cfg.DeviceAccessToken(ctx, resp)
	if err != nil {
		return "", fmt.Errorf("device token: %w", err)
	}

	return tok.AccessToken, nil

	//recv := make(chan callback)
	//
	//go func() {
	//	if err := svc.runHTTPCallbackServer(ctx, cfg, stateParam, verifier, recv); err != nil {
	//		fmt.Println("Error running http callback server:", err)
	//	}
	//}()
	//
	//if err := browser.OpenURL(cfg.AuthCodeURL(stateParam, oauth2.S256ChallengeOption(verifier))); err != nil {
	//	return "", fmt.Errorf("could not open browser: %v", err)
	//}
	//
	//cb := <-recv
	//
	//if cb.err != nil {
	//	return "", fmt.Errorf("id token callback: %v", cb.err)
	//}
	//
	//return cb.accessToken, nil
}

type callback struct {
	accessToken string
	err         error
}

func (svc OIDC) runHTTPCallbackServer(
	ctx context.Context,
	cfg oauth2.Config,
	state string,
	verifier string,
	recv chan callback,
) error {
	var (
		s = http.Server{
			Addr: "localhost:64554",
		}
		mux = http.NewServeMux()
	)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var err error

		defer func() {
			time.AfterFunc(1*time.Second, func() {
				s.Close()
			})
		}()

		if r.URL.Query().Get("state") != state {
			recv <- callback{
				err: fmt.Errorf("state mismatch"),
			}
			return
		}

		code := r.URL.Query().Get("code")

		oauth2Token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			recv <- callback{
				err: fmt.Errorf("failed to exchange code for token: %v", err),
			}
			return
		}

		accessToken, ok := oauth2Token.Extra("access_token").(string)
		if !ok {
			recv <- callback{
				err: fmt.Errorf("no access_token field in oauth2 token"),
			}
			return
		}

		_, _ = w.Write([]byte("Success! You can now close this browser window and return to the terminal."))
		recv <- callback{
			accessToken: accessToken,
		}
	})

	s.Handler = mux

	if err := s.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	return nil
}
