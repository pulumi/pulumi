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
	"fmt"

	"github.com/spf13/cobra"
)

func newEnvTagRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [<org-name>/][<project-name>/]<environment-name> <tag-name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove an environment tag.",
		Long: "Remove an environment tag\n" +
			"\n" +
			"This command removes an environment tag using the tag name.\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the ls command does not accept versions")
			}

			tagIdentifier := args[1]

			if err := env.esc.client.DeleteEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, tagIdentifier); err != nil { //nolint:lll
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Successfully deleted environment tag: %v\n", tagIdentifier)
			return nil
		},
	}

	return cmd
}
