// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvLogCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool

	cmd := &cobra.Command{
		Use:   "log [<org-name>/]<environment-name>[:<revision-or-tag>]",
		Short: "Show revision logs.",
		Long: "Show revision logs\n" +
			"\n" +
			"This command shows the revision logs for an environment. If a revision\n" +
			"or tag is present, the logs will start at the given revision.\n",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			orgName, envName, revisionOrTag, args, err := env.getEnvName(args)
			if err != nil {
				return err
			}
			_ = args

			before := 0
			if revisionOrTag != "" {
				rev, err := env.esc.client.GetRevisionNumber(ctx, orgName, envName, revisionOrTag)
				if err != nil {
					return err
				}
				before = rev + 1
			}

			return env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error {
				count := 500
				for {
					options := client.ListEnvironmentRevisionsOptions{
						Before: &before,
						Count:  &count,
					}
					revisions, err := env.esc.client.ListEnvironmentRevisions(ctx, orgName, envName, options)
					if err != nil {
						return err
					}
					if len(revisions) == 0 {
						break
					}
					before = revisions[len(revisions)-1].Number

					for _, r := range revisions {
						fmt.Fprintf(stdout, "revision %v", r.Number)
						switch len(r.Tags) {
						case 0:
							// OK
						case 1:
							fmt.Fprintf(stdout, " (tag: %v)", r.Tags[0])
						default:
							fmt.Fprintf(stdout, " (tags: %v)", strings.Join(r.Tags, ", "))
						}
						fmt.Fprintln(stdout, "")

						if r.CreatorLogin == "" {
							fmt.Fprintf(stdout, "Author: <unknown>\n")
						} else {
							fmt.Fprintf(stdout, "Author: %v <%v>\n", r.CreatorName, r.CreatorLogin)
						}

						stamp := r.Created
						if utc {
							stamp = stamp.UTC()
						} else {
							stamp = stamp.Local()
						}

						fmt.Fprintf(stdout, "Date:   %v\n", stamp)
						fmt.Fprintf(stdout, "\n")
					}
				}

				return nil
			})
		},
	}

	cmd.Flags().StringVar(&pagerFlag, "pager", "", "the command to use to page through the environment's revisions")
	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
