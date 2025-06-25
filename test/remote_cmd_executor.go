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
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/tools/remotecommand"
)

type RemoteCmdExecutor struct {
}

func (e *RemoteCmdExecutor) Stream(_ remotecommand.StreamOptions) error {
	panic("implement me")
}

func (e *RemoteCmdExecutor) StreamWithContext(ctx context.Context, opts remotecommand.StreamOptions) error {
	t := time.NewTicker(1 * time.Second)
	counter := 0
	for {
		select {
		case <-t.C:
			if counter == 3 {
				_, _ = opts.Stdout.Write([]byte(`Done (30.0s)! For help, type "help"`))
			}
			_, _ = fmt.Fprintf(opts.Stdout, "%d", counter)
			counter++
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
