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

package platformd

import (
	"math/rand/v2"
	"sync"

	"github.com/pkg/errors"
)

var ErrMaxPortTriesReached = errors.New("maximum number of retries reached")

type portAllocator struct {
	portMin   int
	portMax   int
	allocated map[int]bool

	mu sync.Mutex
}

func newPortAllocator(portMin, portMax uint16) *portAllocator {
	return &portAllocator{
		allocated: make(map[int]bool),
		portMin:   int(portMin),
		portMax:   int(portMax),
	}
}

func (a *portAllocator) Allocate() (uint16, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var (
		maxTries = 5
		try      = 0
	)

	for {
		if try >= maxTries {
			return 0, ErrMaxPortTriesReached
		}

		port := rand.IntN(a.portMax-a.portMin) + a.portMin
		if _, ok := a.allocated[port]; ok {
			try++
			continue
		}

		a.allocated[port] = true
		return uint16(port), nil
	}
}

func (a *portAllocator) Free(port uint16) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.allocated, int(port))
}
