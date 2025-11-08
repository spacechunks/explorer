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

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const dexCfg = `
issuer: http://localhost:3081
enablePasswordDB: true

storage:
  type: sqlite3
  config:
    file: /var/dex/dex.db

web:
  http: 0.0.0.0:3081

oauth2:
  passwordConnector: local

staticPasswords:
  - email: test-user@example.com
    username: test-user
    # bcrypt hash for password "password"
    hash: "$2a$12$FaR5JpO4Fqc5wVN/ZK1lSOxo3qFeaAfbFGMloenbNeLduQpCfDjj6"
    userID: "40d27820-b2e3-49ff-bb18-be59cda68db8"

staticClients:
- id: public-functest-client
  name: public-functest-client
  public: true
  redirectURIs:
  - http://localhost:8000/callback
`

type IDP struct {
	Endpoint string
}

func RunIDP(t *testing.T) IDP {
	ctx := context.Background()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  "functests-dex",
			Image: "ghcr.io/dexidp/dex",
			Cmd: []string{
				"dex",
				"serve",
				"/etc/dex/dex.conf",
			},
			ExposedPorts: []string{"3081:3081/tcp"},
			HostConfigModifier: func(cfg *container.HostConfig) {
				cfg.AutoRemove = true
			},
			WaitingFor: wait.ForExposedPort(),
			Files: []testcontainers.ContainerFile{
				{
					Reader:            bytes.NewReader([]byte(dexCfg)),
					ContainerFilePath: "/etc/dex/dex.conf",
					FileMode:          0777,
				},
			},
		},
		Started: true,
		Reuse:   true,
	})
	require.NoError(t, err)

	ip, err := ctr.Host(ctx)
	require.NoError(t, err)

	return IDP{
		Endpoint: "http://" + ip + ":3081",
	}
}

func (i IDP) IDToken(t *testing.T) string {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("scope", "openid profile email")
	form.Set("username", "test-user@example.com")
	form.Set("password", "password")

	req, err := http.NewRequest(http.MethodPost, i.Endpoint+"/token", bytes.NewBufferString(form.Encode()))
	require.NoError(t, err)

	req.Header.Set("Authorization", "Basic cHVibGljLWZ1bmN0ZXN0LWNsaWVudDo=")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	data := struct {
		IDToken string `json:"id_token"`
	}{}

	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	return data.IDToken
}
