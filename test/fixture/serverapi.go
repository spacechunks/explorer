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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
)

const (
	FakeServerAPIAddress = "198.51.100.50"
	FakeServerAPIPort    = 30000
)

type FakeServerAPIServer struct {
	Name     string
	FlavorID string
	State    string
}

type FakeServerAPI struct {
	*httptest.Server

	mu      sync.Mutex
	servers map[string]*FakeServerAPIServer
}

func NewFakeServerAPI(t *testing.T) *FakeServerAPI {
	api := &FakeServerAPI{
		servers: make(map[string]*FakeServerAPIServer),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /server/v1alpha1/servers", api.createServer)
	mux.HandleFunc("GET /server/v1alpha1/servers/{id}", api.getServer)
	mux.HandleFunc("DELETE /server/v1alpha1/servers/{id}", api.deleteServer)

	api.Server = httptest.NewServer(mux)
	t.Cleanup(api.Server.Close)

	return api
}

func (f *FakeServerAPI) createServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		FlavorId string `json:"flavorId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := uuid.NewString()

	f.mu.Lock()
	f.servers[id] = &FakeServerAPIServer{
		Name:     req.Name,
		FlavorID: req.FlavorId,
		State:    "running",
	}
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job": map[string]any{
			"id":         uuid.NewString(),
			"resourceId": id,
			"state":      "completed",
		},
	})
}

func (f *FakeServerAPI) getServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	f.mu.Lock()
	s, ok := f.servers[id]
	f.mu.Unlock()

	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":    "not_found",
			"message": "server not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"server": map[string]any{
			"id":       id,
			"name":     s.Name,
			"type":     "minigame",
			"flavorId": s.FlavorID,
			"state":    s.State,
			"network": map[string]any{
				"address": FakeServerAPIAddress,
				"port":    FakeServerAPIPort,
			},
		},
	})
}

func (f *FakeServerAPI) deleteServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	f.mu.Lock()
	delete(f.servers, id)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job": map[string]any{
			"id":         uuid.NewString(),
			"resourceId": id,
			"state":      "completed",
		},
	})
}
