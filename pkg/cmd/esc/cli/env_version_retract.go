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

	"github.com/spf13/cobra"
)

func newEnvVersionRetractCmd(env *envCommand) *cobra.Command {
	var replacement string
	var reason string

	cmd := &cobra.Command{
		Use:   "retract [<org-name>/][<project-name>/]<environment-name>@<version>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Retract a specific revision of an environment",
		Long: "Retract a specific revision of an environment\n" +
			"\n" +
			"This command retracts a specific revision of an environment. A retracted\n" +
			"revision can no longer be read or opened. Retracting a revision also updates\n" +
			"any tags that point to the retracted revision to instead point to a\n" +
			"replacement revision. If no replacement is specified, the latest non-retracted\n" +
			"revision preceding the revision being retracted is used as the replacement.\n" +
			"\n" +
			"The revision pointed to by the `latest` tag may not be retracted. To retract\n" +
			"the latest revision of an environment, first update the environment with a new\n" +
			"definition.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version == "latest" {
				return errors.New("cannot retract the `latest` revision")
			}
			_ = args

			var replacementRevision *int
			if replacement != "" {
				replacementRef, err := env.getExistingEnvRefWithRelative(ctx, replacement, &ref)
				if err != nil {
					return err
				}

				rev, err := env.esc.client.GetRevisionNumber(
					ctx,
					replacementRef.orgName,
					replacementRef.projectName,
					replacementRef.envName,
					replacementRef.version,
				)
				if err != nil {
					return err
				}
				replacementRevision = &rev
			}

			return env.esc.client.RetractEnvironmentRevision(ctx, ref.orgName, ref.projectName, ref.envName, ref.version, replacementRevision, reason)
		},
	}

	cmd.Flags().StringVar(&replacement, "replace-with", "", "the version to use to replace the retracted revision")
	cmd.Flags().StringVar(&reason, "reason", "", "the reason for the retraction")

	return cmd
}
