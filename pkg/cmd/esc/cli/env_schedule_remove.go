// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newEnvScheduleRemoveCmd(env *envCommand) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "remove [<org-name>/][<project-name>/]<environment-name> <schedule-id>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove an environment scheduled action.",
		Long: "[EXPERIMENTAL] Remove an environment scheduled action\n" +
			"\n" +
			"This command removes the named scheduled action from the environment.\n" +
			"You will be prompted to confirm by typing `remove` unless --yes is passed.\n",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			yes = yes || cmdutil.IsTruthy(os.Getenv(PulumiSkipConfirmationsEnvVar))

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the remove command does not accept versions")
			}

			scheduleID := args[0]
			if scheduleID == "" {
				return errors.New("schedule ID cannot be empty")
			}

			if !yes {
				prompt := fmt.Sprintf("This will permanently remove schedule %q from %s/%s/%s!",
					scheduleID, ref.orgName, ref.projectName, ref.envName)
				if !env.esc.confirmPrompt(prompt, "remove") {
					return errors.New("confirmation declined")
				}
			}

			if err := env.esc.client.DeleteEnvironmentSchedule(ctx, ref.orgName, ref.projectName, ref.envName, scheduleID); err != nil {
				return err
			}

			fmt.Fprintf(env.esc.stdout, "Removed schedule %s from %s/%s/%s\n",
				scheduleID, ref.orgName, ref.projectName, ref.envName)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompts and proceed with removal anyway")

	return cmd
}
