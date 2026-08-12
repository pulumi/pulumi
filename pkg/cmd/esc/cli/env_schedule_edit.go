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
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

func newEnvScheduleEditCmd(env *envCommand) *cobra.Command {
	var (
		cron string
		once string
	)

	cmd := &cobra.Command{
		Use:     "edit [<org-name>/][<project-name>/]<environment-name> <schedule-id>",
		Aliases: []string{"update", "modify"},
		Short:   "Edit an environment scheduled action.",
		Long: "[EXPERIMENTAL] Edit an environment scheduled action\n" +
			"\n" +
			"This command updates the timing of an existing scheduled action. Use --cron to\n" +
			"switch to (or update) a recurring schedule or --once to switch to (or update) a\n" +
			"one-time schedule at a specific time (ISO 8601 / RFC 3339).\n" +
			"\n" +
			"The minimum cron interval is once per day.\n",
		Args: cobra.ExactArgs(2),
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
				return errors.New("the edit command does not accept versions")
			}

			scheduleID := args[0]
			if scheduleID == "" {
				return errors.New("schedule ID cannot be empty")
			}

			switch {
			case cron == "" && once == "":
				return errors.New("exactly one of --cron or --once must be set")
			case cron != "" && once != "":
				return errors.New("only one of --cron or --once may be set")
			}

			if once != "" {
				t, err := time.Parse(time.RFC3339, once)
				if err != nil {
					return fmt.Errorf("--once must be an ISO 8601 / RFC 3339 timestamp: %w", err)
				}
				if !t.After(time.Now()) {
					return errors.New("--once must be a timestamp in the future")
				}
			}

			req := client.UpdateEnvironmentScheduleRequest{
				ScheduleCron: cron,
				ScheduleOnce: once,
			}

			s, err := env.esc.client.UpdateEnvironmentSchedule(ctx, ref.orgName, ref.projectName, ref.envName, scheduleID, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Updated schedule %s for %s/%s/%s\n",
				s.ID, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	cmd.Flags().StringVar(&cron, "cron", "", "a cron expression for a recurring schedule (minimum interval: once daily)")
	cmd.Flags().StringVar(&once, "once", "", "an ISO 8601 / RFC 3339 timestamp in the future for a one-time schedule")

	return cmd
}
