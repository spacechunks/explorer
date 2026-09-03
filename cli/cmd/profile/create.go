/*
A basic matchmaking service for the Chunk Explorer.
Copyright (C) 2026 Yannic Rieger <oss@76k.io>

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
package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spacechunks/explorer/cli"
	"github.com/spacechunks/explorer/cli/fshelper"
	"github.com/spacechunks/explorer/cli/state"
	"github.com/spf13/cobra"
)

func newCreateCommand(_ context.Context, cliCtx cli.Context) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		cfgHome, err := fshelper.ConfigHome()
		if err != nil {
			return fmt.Errorf("could not get config home: %w", err)
		}

		profileName := args[0]

		if profileName == "" {
			return fmt.Errorf("profile name is required")
		}

		if profileName == "default" {
			return fmt.Errorf("the default profile name cannot be used as it's reserved for internal use")
		}

		profilePath := filepath.Join(cfgHome, args[0])

		if err := os.MkdirAll(profilePath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create profile directory: %w", err)
		}

		cpEndpoint, err := cmd.Flags().GetString("control-plane-endpoint")
		if err != nil {
			return fmt.Errorf("control-plane-endpoint: %w", err)
		}

		idpEndpoint, err := cmd.Flags().GetString("idp-issuer-endpoint")
		if err != nil {
			return fmt.Errorf("idp-issuer-endpoint: %w", err)
		}

		idpClientID, err := cmd.Flags().GetString("idp-client-id")
		if err != nil {
			return fmt.Errorf("idp-client-id: %w", err)
		}

		idpScopes, err := cmd.Flags().GetStringArray("idp-scopes")
		if err != nil {
			return fmt.Errorf("idp-scopes: %w", err)
		}

		cfg := state.Config{
			ControlPlaneEndpoint: cpEndpoint,
			IDPIssuerEndpoint:    idpEndpoint,
			IDPClientID:          idpClientID,
			IDPScopes:            idpScopes,
		}

		if err := cli.WriteYAMLFile(cfg, filepath.Join(profilePath, "config.yaml")); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		set, err := cmd.Flags().GetBool("set")
		if err != nil {
			return fmt.Errorf("set: %w", err)
		}

		if !set {
			return nil
		}

		cliCtx.State.SetActiveProfile(profileName)
		return nil
	}

	cmd := &cobra.Command{
		Use:          "create NAME",
		Args:         cobra.ExactArgs(1),
		Short:        "Creates a new CLI profile",
		RunE:         run,
		SilenceUsage: true,
	}

	defaults := state.DefaultConfig
	cmd.Flags().String("control-plane-endpoint", defaults.ControlPlaneEndpoint, "The control-plane endpoint to use")
	cmd.Flags().String("idp-issuer-endpoint", defaults.IDPIssuerEndpoint, "The IDP endpoint to use for authentication")
	cmd.Flags().String("idp-client-id", defaults.IDPIssuerEndpoint, "The client ID to use for authentication with the IDP")
	cmd.Flags().StringArray("idp-scopes", defaults.IDPScopes, "The scopes to use when authenticating with the IDP")
	cmd.Flags().Bool("set", false, "Sets the profile as active")

	return cmd
}
