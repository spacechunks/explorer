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
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/pkg/browser"
	userv1alpha1 "github.com/spacechunks/explorer/api/user/v1alpha1"
	"github.com/spacechunks/explorer/cli/state"
	"golang.org/x/oauth2"
)

type Service interface {
	APIToken(ctx context.Context) (string, error)
	IDToken(ctx context.Context) (string, error)
}

func NewOIDC(
	ctx context.Context,
	state *state.Data,
	clientID string,
	issuerEndpoint string,
	client userv1alpha1.UserServiceClient,
) (*OIDC, error) {
	provider, err := oidc.NewProvider(ctx, issuerEndpoint)
	if err != nil {
		return nil, err
	}

	return &OIDC{
		provider: provider,
		tokenVerifier: provider.Verifier(&oidc.Config{
			ClientID: clientID,
		}),
		clientID:   clientID,
		state:      state,
		userClient: client,
	}, nil
}

type OIDC struct {
	provider      *oidc.Provider
	tokenVerifier *oidc.IDTokenVerifier
	userClient    userv1alpha1.UserServiceClient
	clientID      string
	state         *state.Data
}

func (svc OIDC) APIToken(ctx context.Context) (string, error) {
	if err := validateToken(svc.state.ControlPlaneAPIToken); err != nil {
		// the api token is not valid, so we need a new one.
		// now first check if our id token is still valid.
		var idTok string
		if err := validateToken(svc.state.IDToken); err != nil {
			fmt.Println("GET ID TOKEN")
			idTok, err = svc.getIDToken(ctx)
			if err != nil {
				return "", fmt.Errorf("id token: %w", err)
			}
		}

		// get our api token with the still valid or recently renewed id token
		apiTok, err := svc.getAPIToken(ctx, svc.state.IDToken)
		if err != nil {
			return "", fmt.Errorf("api token: %w", err)
		}

		svc.state.Update(state.Data{
			ControlPlaneAPIToken: apiTok,
			IDToken:              idTok,
		})
	}

	// we got what we need: a still valid or newly issued api token.
	// so, we can return. we don't need to check if the id token is
	// valid, because the only thing we need to the control plane
	// is the api token. once it's expired we'll check the id token
	// again and possibly renew it.
	return svc.state.ControlPlaneAPIToken, nil
}

func (svc OIDC) IDToken(ctx context.Context) (string, error) {
	tok, err := svc.getIDToken(ctx)
	if err != nil {
		return "", fmt.Errorf("id token: %w", err)
	}

	svc.state.Update(state.Data{
		IDToken: tok,
	})

	return tok, nil
}

//type clock struct{}
//
//func (c clock) Now() time.Time {
//	return time.Now().Add(5 * time.Minute)
//}

func validateToken(token string) error {
	tok, err := jwt.ParseString(token, jwt.WithVerify(false))
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("parse api token: %w", err)
	}

	if err := jwt.Validate(tok); err != nil {
		fmt.Println(err)
		return fmt.Errorf("validate api token: %w", err)
	}

	return nil
}

func (svc OIDC) getAPIToken(ctx context.Context, idToken string) (string, error) {
	resp, err := svc.userClient.Login(ctx, &userv1alpha1.LoginRequest{
		IdToken: idToken,
	})
	if err != nil {
		return "", err
	}
	return resp.ApiToken, nil
}

func (svc OIDC) getIDToken(ctx context.Context) (string, error) {
	var (
		cfg = oauth2.Config{
			ClientID:    svc.clientID,
			RedirectURL: "http://localhost:8556",
			Endpoint:    svc.provider.Endpoint(),
			Scopes:      []string{oidc.ScopeOpenID, "profile", "email", "offline_access"},
		}
		verifier   = oauth2.GenerateVerifier()
		stateParam = oauth2.GenerateVerifier()
	)

	recv := make(chan callback)

	go func() {
		if err := svc.runHTTPCallbackServer(ctx, cfg, stateParam, verifier, recv); err != nil {
			fmt.Println("Error running http callback server:", err)
		}
	}()

	if err := browser.OpenURL(cfg.AuthCodeURL(stateParam, oauth2.S256ChallengeOption(verifier))); err != nil {
		return "", fmt.Errorf("could not open browser: %v", err)
	}

	cb := <-recv

	if cb.err != nil {
		return "", fmt.Errorf("id token callback: %v", cb.err)
	}

	return cb.idToken, nil
}

type callback struct {
	idToken string
	err     error
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
			Addr: "localhost:8556",
		}
		mux = http.NewServeMux()
	)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var err error

		defer func() {
			time.AfterFunc(5*time.Second, func() {
				s.Close()
			})
			if err == nil {
				return
			}
			recv <- callback{
				err: err,
			}
			http.Error(w, "An error occured: "+err.Error(), http.StatusInternalServerError)
		}()

		if r.URL.Query().Get("state") != state {
			err = fmt.Errorf("state did not match")
			return
		}

		code := r.URL.Query().Get("code")

		oauth2Token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			err = fmt.Errorf("failed to exchange code for token: %v", err)
			return
		}

		idToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			err = fmt.Errorf("no id_token field in oauth2 token")
			return
		}

		_, err = svc.tokenVerifier.Verify(ctx, idToken)
		if err != nil {
			err = fmt.Errorf("failed to verify id token: %v", err)
			return
		}

		_, _ = w.Write([]byte("Success! You can now close this browser window and return to the terminal."))
		recv <- callback{
			idToken: idToken,
		}
		close(recv)
	})

	s.Handler = mux
	return s.ListenAndServe()
}
