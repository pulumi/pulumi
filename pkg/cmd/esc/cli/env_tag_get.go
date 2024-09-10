// Copyright 2024, Pulumi Corporation.

package cli

import (
	"context"
	"errors"

	"github.com/pulumi/esc/cmd/esc/cli/style"
	"github.com/spf13/cobra"
)

func newEnvTagGetCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "get [<org-name>/][<project-name>/]<environment-name> <name>",
		Args:  cobra.ExactArgs(2),
		Short: "Get an environment tag",
		Long: "Get an environment tag\n" +
			"\n" +
			"This command get a tag with the given name on the specified environment.",
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

			if ref.version != "" {
				return errors.New("the tag command does not accept versions")
			}

			name := args[0]
			if name == "" {
				return errors.New("environment tag name cannot be empty")
			}

			tag, err := env.esc.client.GetEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, name)
			if err != nil {
				return err
			}

			st := style.NewStylist(style.Profile(env.esc.stdout))

			printTag(env.esc.stdout, st, tag, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
