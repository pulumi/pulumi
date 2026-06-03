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

func newEnvScheduleGetCmd(env *envCommand) *cobra.Command {
	var utc bool
	var output string

	cmd := &cobra.Command{
		Use:   "get [<org-name>/][<project-name>/]<environment-name> <schedule-id>",
		Short: "Show details for an environment scheduled action.",
		Long: "[EXPERIMENTAL] Show details for an environment scheduled action\n" +
			"\n" +
			"This command retrieves details for a single scheduled action.\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the get command does not accept versions")
			}

			scheduleID := args[0]
			if scheduleID == "" {
				return errors.New("schedule ID cannot be empty")
			}

			s, err := env.esc.client.GetEnvironmentSchedule(ctx, ref.orgName, ref.projectName, ref.envName, scheduleID)
			if err != nil {
				return err
			}

			if format == outputJSON {
				return writeJSON(env.esc.stdout, newScheduleJSON(*s, utcFlag(utc)))
			}

			printSchedule(env.esc.stdout, *s, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	addOutputFlag(cmd, &output)

	return cmd
}
