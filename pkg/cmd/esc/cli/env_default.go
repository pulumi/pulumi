// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvDefaultCmd(env *envCommand) *cobra.Command {
	var accept bool
	var revoke bool

	cmd := &cobra.Command{
		Use:   "default",
		Args:  cobra.NoArgs,
		Short: "Show the default environment.",
		Long: "Show the default environment\n" +
			"\n" +
			"This command shows the environment that commands operate on when none is given\n" +
			"explicitly, along with the source of the default. See `esc env --help` for the\n" +
			"inference rules.\n" +
			"\n" +
			"An `.esc.yaml` file must be accepted before it takes effect. Interactive commands\n" +
			"prompt on first use and when the file changes; pass `--accept` to accept the file\n" +
			"non-interactively (e.g. in CI) and `--revoke` to forget a prior decision.\n",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if accept || revoke {
				if accept && revoke {
					return errors.New("--accept and --revoke may not be used together")
				}
				dir, err := env.esc.workingDir()
				if err != nil {
					return err
				}
				contents, path, err := env.findDotESC(dir)
				if err != nil {
					return err
				}
				if contents == nil {
					return errors.New("no .esc.yaml found")
				}
				relPath := relativePath(dir, path)
				if accept {
					if err := env.setTrust(path, contents, trustAccept); err != nil {
						return err
					}
					fmt.Fprintf(env.esc.stdout, "Accepted default environment configuration at %v\n", relPath)
				} else {
					if err := env.revokeTrust(path); err != nil {
						return err
					}
					fmt.Fprintf(env.esc.stdout, "Revoked default environment configuration at %v\n", relPath)
				}
				return nil
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			desc, source, err := env.inferDefaultEnv()
			if err != nil {
				return fmt.Errorf("configuring default environment: %w", err)
			}

			switch desc := desc.(type) {
			case nil:
				return errors.New("no default environment")
			case environmentRef:
				fmt.Fprintf(env.esc.stdout, "%v\n", desc.String())
			case importList:
				fmt.Fprintf(env.esc.stdout, "anonymous environment in organization %v\n", desc.orgName)
				fmt.Fprintf(env.esc.stdout, "  imports:\n")
				for _, imp := range desc.imports {
					fmt.Fprintf(env.esc.stdout, "    - %v\n", imp)
				}
			default:
				return fmt.Errorf("unexpected environment desc of type %T", desc)
			}
			fmt.Fprintf(env.esc.stdout, "  source: %v\n", source)
			return nil
		},
	}

	cmd.Flags().BoolVar(&accept, "accept", false, "Accept the nearest .esc.yaml as the default environment")
	cmd.Flags().BoolVar(&revoke, "revoke", false, "Forget a prior decision about the nearest .esc.yaml")

	return cmd
}
