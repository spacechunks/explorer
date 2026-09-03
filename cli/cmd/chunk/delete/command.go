package delete

import (
	"context"
	"fmt"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	"github.com/spacechunks/explorer/cli"
	"github.com/spf13/cobra"
)

func NewCommand(ctx context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("chunk id is missing")
		}

		// TODO: replace by GetChunkRequest with name filter

		_, err := cliCtx.Client.GetChunk(ctx, &chunkv1alpha1.GetChunkRequest{
			Id: args[0],
		})
		if err != nil {
			return fmt.Errorf("error while getting chunk: %w", err)
		}

		if !cli.Prompt(cli.ColorRed + "Are you sure you want to delete your Chunk? This CANNOT be undone! (y/n):" + cli.ColorReset) { //nolint:lll
			fmt.Println("Aborted.")
			return nil
		}

		if !cli.Prompt(cli.ColorRed + "Are you really, really sure? (y/n):" + cli.ColorReset) {
			fmt.Println("Aborted.")
			return nil
		}

		_, err = cliCtx.Client.DeleteChunk(ctx, &chunkv1alpha1.DeleteChunkRequest{
			Id: args[0],
		})
		if err != nil {
			return fmt.Errorf("error while deleting chunk: %w", err)
		}

		return nil
	}

	return &cobra.Command{
		Use:          "delete",
		Short:        "Permanently deletes a Chunk",
		RunE:         run,
		SilenceUsage: true,
	}
}
