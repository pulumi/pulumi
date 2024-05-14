// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/spf13/cobra"
)

func newEnvVersionTagCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "tag [<org-name>/]<environment-name>@<tag> [<revision-number>]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Create or update a tagged version",
		Long: "Create or update a tagged version\n" +
			"\n" +
			"This command creates or updates the tagged version with the given name.\n" +
			"If a revision is passed as the second argument, then the tagged version is\n" +
			"updated to refer to the indicated revision. Otherwise, the tagged version\n" +
			"is updated to point to the latest revision.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			orgName, envName, tagName, args, err := env.getEnvName(args)
			if err != nil {
				return err
			}
			if tagName == "" || isRevisionNumber(tagName) {
				return errors.New("please specify a name for the tagged version")
			}

			var revision int
			if len(args) == 0 {
				latest, err := env.esc.client.GetEnvironmentRevisionTag(ctx, orgName, envName, "latest")
				if err != nil {
					return err
				}
				revision = latest.Revision
			} else {
				revision64, err := strconv.ParseInt(args[0], 10, 0)
				if err != nil {
					return fmt.Errorf("invalid revision number %q: %w", args[0], err)
				}
				revision = int(revision64)
			}

			err = env.esc.client.UpdateEnvironmentRevisionTag(ctx, orgName, envName, tagName, &revision)
			if err == nil {
				return err
			}
			if !client.IsNotFound(err) {
				return err
			}
			return env.esc.client.CreateEnvironmentRevisionTag(ctx, orgName, envName, tagName, &revision)
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
