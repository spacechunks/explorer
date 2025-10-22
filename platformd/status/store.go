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

package status

import (
	"maps"
	"sync"
)

type Store interface {
	Update(id string, status Status)
	Get(id string) *Status
	Del(id string)
	View() map[string]Status
}

func NewMemStore() *MemStore {
	return &MemStore{
		data: make(map[string]Status),
	}
}

type MemStore struct {
	data map[string]Status
	mu   sync.Mutex
}

func (s *MemStore) Update(id string, new Status) {
	s.mu.Lock()
	defer s.mu.Unlock()

	curr, ok := s.data[id]
	if !ok {
		s.data[id] = new
		return
	}

	if curr.WorkloadStatus != nil {
		if new.WorkloadStatus.State != "" {
			curr.WorkloadStatus.State = new.WorkloadStatus.State
		}

		if new.WorkloadStatus.Port != 0 {
			curr.WorkloadStatus.Port = new.WorkloadStatus.Port
		}
		s.data[id] = curr
	}

	if curr.CheckpointStatus != nil {
		if new.CheckpointStatus.State != "" {
			curr.CheckpointStatus.State = new.CheckpointStatus.State
		}

		if new.CheckpointStatus.Port != 0 {
			curr.CheckpointStatus.Port = new.CheckpointStatus.Port
		}

		if new.CheckpointStatus.CompletedAt != nil {
			curr.CheckpointStatus.CompletedAt = new.CheckpointStatus.CompletedAt
		}

		if new.CheckpointStatus.Message != "" {
			curr.CheckpointStatus.Message = new.CheckpointStatus.Message
		}
		s.data[id] = curr
	}
}

func (s *MemStore) Get(id string) *Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status, ok := s.data[id]; ok {
		return &status
	}
	return nil
}

func (s *MemStore) Del(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
}

// View returns a copy of the current state of the underlying map
func (s *MemStore) View() map[string]Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	cpy := make(map[string]Status, len(s.data))
	maps.Copy(cpy, s.data)

	return cpy
}
