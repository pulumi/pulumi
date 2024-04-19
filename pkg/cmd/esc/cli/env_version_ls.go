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

func newEnvVersionLsCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool

	cmd := &cobra.Command{
		Use:   "ls [<org-name>/]<environment-name>",
		Short: "List version tags.",
		Long: "List version tags\n" +
			"\n" +
			"This command lists the version tags for an environment.\n",
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
			if revisionOrTag != "" {
				return fmt.Errorf("the ls command does not accept revisions or tags")
			}
			_ = args

			st := style.NewStylist(style.Profile(env.esc.stdout))

			after := ""
			return env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error {
				count := 500
				for {
					options := client.ListEnvironmentRevisionTagsOptions{
						After: after,
						Count: &count,
					}
					tags, err := env.esc.client.ListEnvironmentRevisionTags(ctx, orgName, envName, options)
					if err != nil {
						return err
					}
					if len(tags) == 0 {
						break
					}
					after = tags[len(tags)-1].Name

					for _, t := range tags {
						printRevisionTag(stdout, st, t, utc)
						fmt.Fprintf(stdout, "\n")
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&pagerFlag, "pager", "", "the command to use to page through the environment's version tags")
	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
