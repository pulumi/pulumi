// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/style"
)

func newEnvTagLsCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool
	var output string

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name>",
		Aliases: []string{"ls"},
		Short:   "List environment tags.",
		Long: "List environment tags\n" +
			"\n" +
			"This command lists an environment's tags.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the ls command does not accept versions")
			}

			st := style.NewStylist(style.Profile(env.esc.stdout))

			allTags := []*client.EnvironmentTag{}
			after := ""
			err = env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error { //nolint:lll
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
			if err != nil {
				return err
			}

			sort.Slice(allTags, func(a, b int) bool {
				return strings.ToLower(allTags[a].Name) < strings.ToLower(allTags[b].Name)
			})

			if format == outputJSON {
				out := struct {
					Tags []tagJSON `json:"tags"`
				}{Tags: make([]tagJSON, 0, len(allTags))}
				for _, t := range allTags {
					out.Tags = append(out.Tags, newTagJSON(t, utcFlag(utc)))
				}
				return writeJSON(env.esc.stdout, out)
			}

			for _, t := range allTags {
				printTag(env.esc.stdout, st, t, utcFlag(utc))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pagerFlag, "pager", "", "the command to use to page through the environment's version tags")
	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	addOutputFlag(cmd, &output)

	return cmd
}
