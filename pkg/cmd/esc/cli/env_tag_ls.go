// Copyright 2024, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/style"
)

func newEnvTagLsCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool

	cmd := &cobra.Command{
		Use:   "ls [<org-name>/][<project-name>/]<environment-name>",
		Short: "List environment tags.",
		Long: "List environment tags\n" +
			"\n" +
			"This command lists an environment's tags.\n",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the ls command does not accept versions")
			}

			st := style.NewStylist(style.Profile(env.esc.stdout))

			allTags := []*client.EnvironmentTag{}
			after := ""
			err = env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error {
				count := 500
				for {
					options := client.ListEnvironmentTagsOptions{
						After: after,
						Count: &count,
					}
					tags, next, err := env.esc.client.ListEnvironmentTags(ctx, ref.orgName, ref.projectName, ref.envName, options)
					if err != nil {
						return err
					}

					after = next

					allTags = append(allTags, tags...)

					if after == "0" {
						break
					}
				}
				return nil
			})

			sort.Slice(allTags, func(a, b int) bool {
				return strings.ToLower(allTags[a].Name) < strings.ToLower(allTags[b].Name)
			})

			for _, t := range allTags {
				printTag(env.esc.stdout, st, t, utcFlag(utc))
			}
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pagerFlag, "pager", "", "the command to use to page through the environment's version tags")
	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")

	return cmd
}
