// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/style"
)

func newEnvVersionHistoryCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool

	cmd := &cobra.Command{
		Use:   "history [<org-name>/][<project-name>/]<environment-name>[@<version>]",
		Short: "Show revision history.",
		Long: "Show revision history\n" +
			"\n" +
			"This command shows the revision history for an environment. If a version\n" +
			"is present, the logs will start at the corresponding revision.\n",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			_ = args

			before := 0
			if ref.version != "" {
				rev, err := env.esc.client.GetRevisionNumber(ctx, ref.orgName, ref.projectName, ref.envName, ref.version)
				if err != nil {
					return err
				}
				before = rev + 1
			}

			// NOTE: we use the color profile from the user-visible stdout rather than the color profile from the pager's stdout.
			st := style.NewStylist(style.Profile(env.esc.stdout))
			return env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error {
				count := 500
				for {
					options := client.ListEnvironmentRevisionsOptions{
						Before: &before,
						Count:  &count,
					}
					revisions, err := env.esc.client.ListEnvironmentRevisions(ctx, ref.orgName, ref.projectName, ref.envName, options)
					if err != nil {
						return err
					}
					if len(revisions) == 0 {
						break
					}
					before = revisions[len(revisions)-1].Number

					for _, r := range revisions {
						printRevision(stdout, st, r, utcFlag(utc))
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
