// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/style"
	"github.com/spf13/cobra"
)

func newEnvVersionCmd(env *envCommand) *cobra.Command {
	var utc bool

	cmd := &cobra.Command{
		Use:   "version [<org-name>/][<project-name>/]<environment-name>@<version>",
		Args:  cobra.ExactArgs(1),
		Short: "Manage versions",
		Long: "Manage versions\n" +
			"\n" +
			"This command describes the referenced environment version.\n" +
			"\n" +
			"Subcommands exist for viewing revision history and managing" +
			"tagged versions.",
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
			if ref.version == "" {
				return errors.New("please specify a version")
			}
			_ = args

			st := style.NewStylist(style.Profile(env.esc.stdout))
			if isRevisionNumber(ref.version) {
				revisionNumber, err := strconv.ParseInt(ref.version, 10, 0)
				if err != nil {
					return err
				}
				rev, err := env.esc.client.GetEnvironmentRevision(ctx, ref.orgName, ref.projectName, ref.envName, int(revisionNumber))
				if err != nil {
					return err
				}
				printRevision(env.esc.stdout, st, *rev, utcFlag(utc))
			} else {
				tag, err := env.esc.client.GetEnvironmentRevisionTag(ctx, ref.orgName, ref.projectName, ref.envName, ref.version)
				if err != nil {
					return err
				}
				printRevisionTag(env.esc.stdout, st, *tag, utcFlag(utc))
			}
			return nil
		},
	}

	cmd.AddCommand(newEnvVersionTagCmd(env))
	cmd.AddCommand(newEnvVersionHistoryCmd(env))
	cmd.AddCommand(newEnvVersionRetractCmd(env))
	cmd.AddCommand(newEnvVersionRollbackCmd(env))

	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}

type utcFlag bool

func (utc utcFlag) time(t time.Time) time.Time {
	if utc {
		return t.UTC()
	}
	return t.Local()
}

func printRevision(stdout io.Writer, st *style.Stylist, r client.EnvironmentRevision, utc utcFlag) {
	rules := style.Default()

	st.Fprintf(stdout, rules.H1.StylePrimitive, "revision %v", r.Number)
	if r.Retracted != nil {
		st.Fprintf(stdout, rules.CodeBlock.Chroma.GenericDeleted, " (retracted)")
	} else {
		switch len(r.Tags) {
		case 0:
			// OK
		case 1:
			st.Fprintf(stdout, rules.LinkText, " (tag: %v)", r.Tags[0])
		default:
			st.Fprintf(stdout, rules.LinkText, " (tags: %v)", strings.Join(r.Tags, ", "))
		}
	}
	fmt.Fprintln(stdout, "")

	if r.CreatorLogin == "" {
		fmt.Fprintf(stdout, "Author: <unknown>\n")
	} else {
		fmt.Fprintf(stdout, "Author: %v <%v>\n", r.CreatorName, r.CreatorLogin)
	}

	fmt.Fprintf(stdout, "Date:   %v\n", utc.time(r.Created))

	if r.Retracted != nil {
		if r.Retracted.ByLogin == "" {
			fmt.Fprintf(stdout, "\n    Retracted by <unknown>")
		} else {
			fmt.Fprintf(stdout, "\n    Retracted by %v <%v>", r.Retracted.ByName, r.Retracted.ByLogin)
		}

		fmt.Fprintf(stdout, " at %v and replaced with revision %v.\n", utc.time(r.Retracted.At), r.Retracted.Replacement)

		if r.Retracted.Reason != "" {
			fmt.Fprintln(stdout, "")

			reason := strings.Split(r.Retracted.Reason, "\n")
			for _, line := range reason {
				fmt.Fprintf(stdout, "    %v\n", line)
			}
		}
	}
}

func printRevisionTag(stdout io.Writer, st *style.Stylist, tag client.EnvironmentRevisionTag, utc utcFlag) {
	rules := style.Default()

	st.Fprintf(stdout, rules.LinkText, "%v\n", tag.Name)
	fmt.Fprintf(stdout, "Revision %v\n", tag.Revision)

	fmt.Fprintf(stdout, "Last updated at %v by ", utc.time(tag.Modified))
	if tag.EditorLogin == "" {
		fmt.Fprintf(stdout, "<unknown>")
	} else {
		fmt.Fprintf(stdout, "%v <%v>", tag.EditorName, tag.EditorLogin)
	}
	fmt.Fprintln(stdout)
}
