// Copyright 2024, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/style"
	"github.com/spf13/cobra"
)

func newEnvTagCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "tag [<org-name>/][<project-name>/]<environment-name> <name> <value>",
		Args:  cobra.ExactArgs(3),
		Short: "Manage environment tags",
		Long: "Manage environment tags\n" +
			"\n" +
			"This command creates a tag with the given name on the specified environment.\n" +
			"\n" +
			"Subcommands exist for reading, listing, updating, and removing tags.",
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
			value := args[1]
			if name == "" {
				return errors.New("environment tag name cannot be empty")
			}
			if value == "" {
				return errors.New("environment tag value cannot be empty")
			}

			tag, err := env.esc.client.GetEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, name)
			if err != nil && !client.IsNotFound(err) {
				return err
			}

			st := style.NewStylist(style.Profile(env.esc.stdout))

			if tag != nil {
				if tag.Name == name && tag.Value == value {
					printTag(env.esc.stdout, st, tag, utcFlag(utc))
					return nil
				}

				t, err := env.esc.client.UpdateEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, tag.Name, tag.Value, tag.Name, value)
				if err == nil {
					printTag(env.esc.stdout, st, t, utcFlag(utc))
					return nil
				}
				return err
			}

			t, err := env.esc.client.CreateEnvironmentTag(ctx, ref.orgName, ref.projectName, ref.envName, name, value)
			if err != nil {
				return err
			}

			printTag(env.esc.stdout, st, t, utcFlag(utc))

			return nil
		},
	}

	cmd.AddCommand(newEnvTagGetCmd(env))
	cmd.AddCommand(newEnvTagLsCmd(env))
	cmd.AddCommand(newEnvTagRmCmd(env))
	cmd.AddCommand(newEnvTagMvCmd(env))

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}

func printTag(stdout io.Writer, st *style.Stylist, tag *client.EnvironmentTag, utc utcFlag) {
	rules := style.Default()

	st.Fprintf(stdout, rules.LinkText, "Name: %v\n", tag.Name)
	st.Fprintf(stdout, rules.LinkText, "Value: %v\n", tag.Value)

	fmt.Fprintf(stdout, "Last updated at %v by ", utc.time(tag.Modified))
	if tag.EditorLogin == "" {
		fmt.Fprintf(stdout, "<unknown>")
	} else {
		fmt.Fprintf(stdout, "%v <%v>", tag.EditorName, tag.EditorLogin)
	}
	fmt.Fprintln(stdout)
}
