// Copyright 2024, Pulumi Corporation.

package cli

import (
	"context"
	"errors"

	"github.com/pulumi/esc/cmd/esc/cli/style"
	"github.com/spf13/cobra"
)

func newEnvTagMvCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "mv [<org-name>/][<project-name>/]<environment-name> <name> <new-name>",
		Args:  cobra.ExactArgs(3),
		Short: "Move an environment tag",
		Long: "Move an environment tag\n" +
			"\n" +
			"This command updates a tag with the given name on the specified environment, " +
			"changing it's name.\n",
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
			newName := args[1]
			if name == "" {
				return errors.New("environment tag name cannot be empty")
			}
			if newName == "" {
				return errors.New("environment tag value cannot be empty")
			}

			tag, err := env.esc.client.GetEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, name)
			if err != nil {
				return err
			}

			st := style.NewStylist(style.Profile(env.esc.stdout))

			if tag.Name == newName {
				printTag(env.esc.stdout, st, tag, utcFlag(utc))
				return nil
			}

			t, err := env.esc.client.UpdateEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, tag.Name, tag.Value, newName, tag.Value)
			if err != nil {
				return err
			}

			printTag(env.esc.stdout, st, t, utcFlag(utc))
			return nil
		},
	}

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
