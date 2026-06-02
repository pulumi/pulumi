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

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/style"
)

func newEnvTagMvCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "mv [<org-name>/][<project-name>/]<environment-name> <name> <new-name>",
		Args:  cobra.ExactArgs(3),
		Short: "Move an environment tag",
		Long: "Move an environment tag\n" +
			"\n" +
			"This command updates a tag with the given name on the specified environment, " +
			"changing it's name.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the tag command does not accept versions")
			}

			name := args[0]
			newName := args[1]
			if name == "" {
				return errors.New("environment tag name cannot be empty")
			}
			if newName == "" {
				return errors.New("environment tag value cannot be empty")
			}

			tag, err := env.esc.client.GetEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, name)
			if err != nil {
				return err
			}

			st := style.NewStylist(style.Profile(env.esc.stdout))

			if tag.Name == newName {
				printTag(env.esc.stdout, st, tag, utcFlag(utc))
				return nil
			}

			t, err := env.esc.client.UpdateEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, tag.Name, tag.Value, newName, tag.Value)
			if err != nil {
				return err
			}

			printTag(env.esc.stdout, st, t, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
