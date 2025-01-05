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

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"

	"github.com/spacechunks/platform/nodedev"
	"github.com/spacechunks/platform/test"
	"github.com/stretchr/testify/require"
)

func before(t *testing.T, ctx context.Context, testEnv *test.Env) {
	t.Cleanup(func() {
		testEnv.Cleanup(ctx)
	})
	testEnv.Setup(ctx)
	addr := testEnv.CreateServer(ctx)
	test.WaitServerReady(t, "unix", addr+":22", 1*time.Minute) // as soon as we can ssh we are ready
	setup(t, ctx, testEnv, addr)
}

func TestPTPNAT(t *testing.T) {
	var (
		testEnv = test.NewEnv(t, os.Getenv("E2E_HCLOUD_TOKEN"))
		ctx     = context.Background()
	)
	before(t, ctx, testEnv)

	resp, err := http.Get("http://" + testEnv.Servers[0].PublicNet.IPv4.IP.String() + ":80")
	require.NoError(t, err)

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func setup(t *testing.T, ctx context.Context, env *test.Env, addr string) {
	config, err := auth.PrivateKey(env.SSHUser(), env.PrivateKeyPath(), ssh.InsecureIgnoreHostKey())
	require.NoError(t, err)

	sshClient, err := ssh.Dial("tcp", addr+":22", &config)
	require.NoError(t, err)

	defer sshClient.Close()

	scpClient, err := scp.NewClientBySSH(sshClient)
	require.NoError(t, err)

	defer scpClient.Close()

	err = fs.WalkDir(nodedev.Files, ".", func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}

		data, err := nodedev.Files.ReadFile(path)
		if err != nil {
			return err
		}

		if err := scpClient.CopyFile(ctx, bytes.NewReader(data), path, "0777"); err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err)

	sess, err := sshClient.NewSession()
	require.NoError(t, err)

	defer sess.Close()

	out, err := sess.CombinedOutput("./provision-cni.sh")
	if err != nil {
		fmt.Println(string(out))
		fmt.Println("===")
		t.Fatal(err)
	}
}
