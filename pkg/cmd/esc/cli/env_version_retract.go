// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
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
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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
