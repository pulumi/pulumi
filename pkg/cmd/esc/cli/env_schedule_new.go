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

func newEnvScheduleNewCmd(env *envCommand) *cobra.Command {
	var (
		cron string
		once string
	)

	cmd := &cobra.Command{
		Use:   "new [<org-name>/][<project-name>/]<environment-name>",
		Short: "Create a new scheduled action on an environment.",
		Long: "[EXPERIMENTAL] Create a new scheduled action on an environment\n" +
			"\n" +
			"This command schedules a secret rotation against the environment. Use --cron to\n" +
			"schedule a recurring rotation or --once to schedule a single rotation at a\n" +
			"specific time (ISO 8601 / RFC 3339).\n" +
			"\n" +
			"Only one schedule per environment is currently supported; creating a second\n" +
			"schedule will fail. The minimum cron interval is once per day.\n",
		Args: cobra.ExactArgs(1),
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
				return errors.New("the new command does not accept versions")
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

			existing, err := env.esc.client.ListEnvironmentSchedules(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}
			if existing != nil && len(existing.Schedules) > 0 {
				return fmt.Errorf(
					"environment %s/%s/%s already has a schedule (%s); only one schedule per environment is currently supported",
					ref.orgName,
					ref.projectName,
					ref.envName,
					existing.Schedules[0].ID,
				)
			}

			req := client.CreateEnvironmentScheduleRequest{
				ScheduleCron:          cron,
				ScheduleOnce:          once,
				SecretRotationRequest: &client.CreateEnvironmentSecretRotationScheduleRequest{},
			}

			s, err := env.esc.client.CreateEnvironmentSchedule(ctx, ref.orgName, ref.projectName, ref.envName, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Created schedule %s for %s/%s/%s\n",
				s.ID, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	cmd.Flags().
		StringVar(&cron, "cron", "", "a cron expression for a recurring schedule (minimum interval: once daily)")
	cmd.Flags().StringVar(&once, "once", "", "an ISO 8601 / RFC 3339 timestamp in the future for a one-time schedule")

	return cmd
}
