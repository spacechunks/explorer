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

package checkpoint

import (
	"maps"
	"sync"
	"time"
)

type Status struct {
	State       State
	Message     string
	CompletedAt time.Time
}

type State string

const (
	StateRunning                   State = "RUNNING"
	StatePullBaseImageFailed       State = "PULL_BASE_IMAGE_FAILED"
	StateContainerWaitReadyFailed  State = "CONTAINER_WAIT_READY_FAILED"
	StateContainerCheckpointFailed State = "CONTAINER_CHECKPOINT_FAILED"
	StatePushCheckpointFailed      State = "PUSH_CHECKPOINT_FAILED"
	StateCompleted                 State = "COMPLETED"
)

// TODO: this is basically copy pasta from workload status store.
//       at some point evaluate if a single status store solution
//       is possible.

type statusStore interface {
	Get(id string) *Status
	Update(id string, status Status)
	View() map[string]Status
	Del(id string)
}

func newStore() statusStore {
	return &inmemStore{
		data: make(map[string]Status),
	}
}

type inmemStore struct {
	data map[string]Status
	mu   sync.Mutex
}

func (s *inmemStore) Update(id string, new Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = new
}

func (s *inmemStore) Get(id string) *Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status, ok := s.data[id]; ok {
		return &status
	}
	return nil
}

func (s *inmemStore) Del(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
}

// View returns a copy of the current state of the underlying map
func (s *inmemStore) View() map[string]Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	cpy := make(map[string]Status, len(s.data))
	maps.Copy(cpy, s.data)

	return cpy
}
