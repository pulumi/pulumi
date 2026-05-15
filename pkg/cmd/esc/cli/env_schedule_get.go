// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
)

func newEnvScheduleGetCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "get [<org-name>/][<project-name>/]<environment-name> <schedule-id>",
		Short: "Show details for an environment scheduled action.",
		Long: "[EXPERIMENTAL] Show details for an environment scheduled action\n" +
			"\n" +
			"This command retrieves details for a single scheduled action.\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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

			printSchedule(env.esc.stdout, *s, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
