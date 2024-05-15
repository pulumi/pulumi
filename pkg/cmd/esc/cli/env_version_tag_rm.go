// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"
)

func newEnvVersionTagRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [<org-name>/]<environment-name>@<tag>",
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

			orgName, envName, version, args, err := env.getEnvName(args)
			if err != nil {
				return err
			}
			if version == "" || isRevisionNumber(version) {
				return errors.New("please specify a tagged version to remove")
			}
			_ = args

			return env.esc.client.DeleteEnvironmentRevisionTag(ctx, orgName, envName, version)
		},
	}

	return cmd
}

func isRevisionNumber(version string) bool {
	_, err := strconv.ParseInt(version, 10, 0)
	return err == nil
}
