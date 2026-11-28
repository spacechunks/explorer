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

package serverapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type State string

const (
	StatePending  State = "pending"
	StateCreating State = "creating"
	StateRunning  State = "running"
	StateUpdating State = "updating"
	StateDeleting State = "deleting"
	StateError    State = "error"
)

type Network struct {
	Address string
	Port    uint16
}

type Status struct {
	State   State
	Network *Network
}

type Client interface {
	CreateServer(ctx context.Context, name string, flavorID string) (string, error)
	GetServer(ctx context.Context, id string) (Status, error)
	DeleteServer(ctx context.Context, id string) error
}

type httpClient struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) Client {
	return &httpClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{},
	}
}

type serverType string

const serverTypeMinigame serverType = "minigame"

type createServerRequest struct {
	Type     serverType `json:"type"`
	Name     string     `json:"name"`
	FlavorId string     `json:"flavorId"`
}

type apiNetwork struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type apiServer struct {
	State   string      `json:"state"`
	Network *apiNetwork `json:"network,omitempty"`
}

type getServerResponse struct {
	Server apiServer `json:"server"`
}

type apiJob struct {
	ResourceId string `json:"resourceId"`
}

type jobCreateResponse struct {
	Job apiJob `json:"job"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (c *httpClient) CreateServer(ctx context.Context, name string, flavorID string) (string, error) {
	var resp jobCreateResponse
	if _, err := c.do(ctx, http.MethodPost, "/server/v1alpha1/servers", createServerRequest{
		Type:     serverTypeMinigame,
		Name:     name,
		FlavorId: flavorID,
	}, &resp); err != nil {
		return "", err
	}

	return resp.Job.ResourceId, nil
}

func (c *httpClient) GetServer(ctx context.Context, id string) (Status, error) {
	var resp getServerResponse
	if _, err := c.do(ctx, http.MethodGet, "/server/v1alpha1/servers/"+id, nil, &resp); err != nil {
		return Status{}, err
	}

	status := Status{
		State: State(resp.Server.State),
	}

	if resp.Server.Network != nil {
		status.Network = &Network{
			Address: resp.Server.Network.Address,
			Port:    uint16(resp.Server.Network.Port),
		}
	}

	return status, nil
}

func (c *httpClient) DeleteServer(ctx context.Context, id string) error {
	statusCode, err := c.do(ctx, http.MethodDelete, "/server/v1alpha1/servers/"+id, nil, nil)
	if err != nil && statusCode != http.StatusNotFound {
		return err
	}
	return nil
}

func (c *httpClient) do(ctx context.Context, method string, path string, body any, out any) (int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return resp.StatusCode, fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}
		return resp.StatusCode, fmt.Errorf(
			"unexpected status code %d: %s (%s)",
			resp.StatusCode,
			apiErr.Message,
			apiErr.Type,
		)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}

	return resp.StatusCode, nil
}
