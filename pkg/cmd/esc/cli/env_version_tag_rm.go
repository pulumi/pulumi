// Copyright 2023, Pulumi Corporation.
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

func newEnvVersionTagRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [<org-name>/][<project-name>/]<environment-name>@<tag>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a tagged version.",
		Long: "Remove a tagged version\n" +
			"\n" +
			"This command removes the tagged version with the given name",
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
				return errors.New("please specify a tagged version to remove")
			}
			_ = args

			return env.esc.client.DeleteEnvironmentRevisionTag(ctx, ref.orgName, ref.projectName, ref.envName, ref.version)
		},
	}

	return cmd
}

func isRevisionNumber(version string) bool {
	return len(version) > 0 && version[0] >= '0' && version[0] <= '9'
}
