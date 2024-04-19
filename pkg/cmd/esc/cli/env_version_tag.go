// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/style"
	"github.com/spf13/cobra"
)

func newEnvVersionTagCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "tag [<org-name>/]<environment-name>:<tag> [revision-or-tag]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Create, describe, or update a version tag.",
		Long: "Create, describe, or update a version tag\n" +
			"\n" +
			"This command creates, describes, or updates the version tag with the given name.\n" +
			"If a revision or tag is passed as the second argument, then the target tag is\n" +
			"updated to refer to the indicated revision.\n",
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
				return errors.New("please specify a tag name")
			}

			if len(args) == 0 {
				tag, err := env.esc.client.GetEnvironmentRevisionTag(ctx, orgName, envName, tagName)
				if err != nil {
					return err
				}

				st := style.NewStylist(style.Profile(env.esc.stdout))
				printRevisionTag(env.esc.stdout, st, *tag, utc)
				return nil
			}

			revision64, err := strconv.ParseInt(args[0], 10, 0)
			if err != nil {
				return fmt.Errorf("invalid revision number %q: %w", args[0], err)
			}
			revision := int(revision64)

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

func printRevisionTag(stdout io.Writer, st *style.Stylist, tag client.EnvironmentRevisionTag, utc bool) {
	rules := style.Default()

	st.Fprintf(stdout, rules.LinkText, "%v\n", tag.Name)
	fmt.Fprintf(stdout, "Revision %v\n", tag.Revision)

	stamp := tag.Modified
	if utc {
		stamp = stamp.UTC()
	} else {
		stamp = stamp.Local()
	}

	fmt.Fprintf(stdout, "Last updated at %v by ", stamp)
	if tag.EditorLogin == "" {
		fmt.Fprintf(stdout, "<unknown>")
	} else {
		fmt.Fprintf(stdout, "%v <%v>", tag.EditorName, tag.EditorLogin)
	}
	fmt.Fprintln(stdout)
}
