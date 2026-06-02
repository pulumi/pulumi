// Copyright 2024, Pulumi Corporation.
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
	"strings"

	"github.com/spf13/cobra"

	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvVersionTagCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "tag [<org-name>/][<project-name>/]<environment-name>@<tag> [@<version>]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Manage tagged versions",
		Long: "Manage tagged versions\n" +
			"\n" +
			"This command creates or updates the tagged version with the given name.\n" +
			"If a revision is passed as the second argument, then the tagged version is\n" +
			"updated to refer to the indicated revision. Otherwise, the tagged version\n" +
			"is updated to point to the latest revision.\n" +
			"\n" +
			"Subcommands exist for listing and removing tagged versions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version == "" || isRevisionNumber(ref.version) {
				return errors.New("please specify a name for the tagged version")
			}

			var revision int
			if len(args) == 0 {
				latest, err := env.esc.client.GetEnvironmentRevisionTag(ctx, ref.orgName, ref.projectName, ref.envName, "latest")
				if err != nil {
					return err
				}
				revision = latest.Revision
			} else {
				version, _ := strings.CutPrefix(args[0], "@")
				revision, err = env.esc.client.GetRevisionNumber(ctx, ref.orgName, ref.projectName, ref.envName, version)
				if err != nil {
					return err
				}
			}

			err = env.esc.client.UpdateEnvironmentRevisionTag(ctx, ref.orgName, ref.projectName, ref.envName, ref.version, &revision)
			if err == nil {
				return err
			}
			if !client.IsNotFound(err) {
				return err
			}
			return env.esc.client.CreateEnvironmentRevisionTag(ctx, ref.orgName, ref.projectName, ref.envName, ref.version, &revision)
		},
	}

	cmd.AddCommand(newEnvVersionTagLsCmd(env))
	cmd.AddCommand(newEnvVersionTagRmCmd(env))

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
