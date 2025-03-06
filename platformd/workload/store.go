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

package workload

import "sync"

type StatusStore interface {
	Update(id string, status Status)
	Get(id string) *Status
}

func NewStore() StatusStore {
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

	curr, ok := s.data[id]
	if !ok {
		s.data[id] = new
		return
	}

	if curr.State != "" {
		curr.State = new.State
	}

	if curr.Port != 0 {
		curr.Port = new.Port
	}
}

func (s *inmemStore) Get(id string) *Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status, ok := s.data[id]; ok {
		return &status
	}
	return nil
}
