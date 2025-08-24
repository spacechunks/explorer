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

package run

import (
	"context"
	"errors"
	"fmt"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spf13/cobra"
)

func NewCommand(ctx context.Context, state cli.State) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New("chunk id and flavor name required")
		}

		var (
			chunkID    = args[0]
			flavorName = args[1]
		)

		c, err := state.Client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
			Id: chunkID,
		})
		if err != nil {
			return fmt.Errorf("failed to get chunk: %w", err)
		}

		f := cli.FindFlavor(c.Chunk.Flavors, func(f *chunkv1alpha1.Flavor) bool {
			return f.Name == flavorName
		})

		if f == nil {
			return fmt.Errorf("flavor %s not found", flavorName)
		}

		// TODO: find flavor

		resp, err := state.InstanceClient.RunChunk(ctx, &instancev1alpha1.RunChunkRequest{
			ChunkId:  c.Chunk.Id,
			FlavorId: f.Id,
		})
		if err != nil {
			return fmt.Errorf("failed to run chunk: %w", err)
		}

		fmt.Printf("Address: %s:%d\n", resp.Instance.Ip, resp.Instance.Port)
		fmt.Printf("State: %s\n", resp.Instance.State)

		return nil
	}

	return &cobra.Command{
		Use:   "run",
		Short: "Run a Chunk flavor",
		RunE:  run,
	}
}
