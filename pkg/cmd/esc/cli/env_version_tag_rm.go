// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
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
