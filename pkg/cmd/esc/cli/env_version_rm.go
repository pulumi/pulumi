// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"
)

func newEnvVersionRmCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [<org-name>/]<environment-name>:<tag>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a version tag.",
		Long: "Remove a version tag\n" +
			"\n" +
			"This command removes the version tag with the given name to refer to the\n" +
			"indicated revision.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			orgName, envName, revisionOrTag, args, err := env.getEnvName(args)
			if err != nil {
				return err
			}
			if revisionOrTag == "" || isRevisionNumber(revisionOrTag) {
				return errors.New("please specify a tag name to remove")
			}
			_ = args

			return env.esc.client.DeleteEnvironmentRevisionTag(ctx, orgName, envName, revisionOrTag)
		},
	}

	return cmd
}

func isRevisionNumber(revisionOrTag string) bool {
	_, err := strconv.ParseInt(revisionOrTag, 10, 0)
	return err == nil
}
