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

package test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"maps"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func RandHexStr(t *testing.T) string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		t.Fatalf("failed reading random bytes: %v", err)
	}
	return fmt.Sprintf("%x", bytes)
}

// WaitServerReady waits until a process, usually some kind of server, can
// accept connections. Fails after no successful connection could be established
// after the timeout.
func WaitServerReady(t *testing.T, network, addr string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		conn, err := net.DialTimeout(network, addr, 1*time.Second)
		if err == nil {
			conn.Close()
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s did not respond within %v", addr, timeout)
		case <-time.After(2 * time.Second):
			continue
		}
	}
}

func NewUUIDv7(t *testing.T) string {
	id, err := uuid.NewV7()
	require.NoError(t, err)
	return id.String()
}

func CreateResourcePackZip(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	var (
		buf = &bytes.Buffer{}
		zw  = zip.NewWriter(buf)
	)

	// sort paths alphabetically, because zip files created by
	// walking a directory on the filesystem also has entries
	// ordered this way. removing this will break CreateResourcePackWorker
	// tests.
	paths := slices.Collect(maps.Keys(files))
	slices.Sort(paths)

	for _, p := range paths {
		w, err := zw.Create(p)
		require.NoError(t, err)

		_, err = w.Write([]byte(files[p]))
		require.NoError(t, err)
	}

	err := zw.Close()
	require.NoError(t, err)

	return buf, fmt.Sprintf("%x", sha1.Sum(buf.Bytes()))
}
