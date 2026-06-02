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
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/style"
	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvVersionHistoryCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool
	var output string

	cmd := &cobra.Command{
		Use:   "history [<org-name>/][<project-name>/]<environment-name>[@<version>]",
		Short: "Show revision history.",
		Long: "Show revision history\n" +
			"\n" +
			"This command shows the revision history for an environment. If a version\n" +
			"is present, the logs will start at the corresponding revision.\n",
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

			revisions, err := listAllEnvironmentRevisions(ctx, env.esc.client, ref, before)
			if err != nil {
				return err
			}

			if format == outputJSON {
				return writeJSON(env.esc.stdout, struct {
					Revisions []client.EnvironmentRevision `json:"revisions"`
				}{revisions})
			}

			// NOTE: we use the color profile from the user-visible stdout rather than the color profile from the pager's stdout.
			st := style.NewStylist(style.Profile(env.esc.stdout))
			return env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error {
				for _, r := range revisions {
					printRevision(stdout, st, r, utcFlag(utc))
					fmt.Fprintf(stdout, "\n")
				}
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&pagerFlag, "pager", "", "the command to use to page through the environment's revisions")
	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	addOutputFlag(cmd, &output)

	return cmd
}

// listAllEnvironmentRevisions pages through every revision strictly before `before`
// (use 0 to start at the latest). All pages are accumulated and returned together.
func listAllEnvironmentRevisions(
	ctx context.Context,
	c client.Client,
	ref environmentRef,
	before int,
) ([]client.EnvironmentRevision, error) {
	all := []client.EnvironmentRevision{}
	count := 500
	for {
		options := client.ListEnvironmentRevisionsOptions{
			Before: &before,
			Count:  &count,
		}
		revisions, err := c.ListEnvironmentRevisions(ctx, ref.orgName, ref.projectName, ref.envName, options)
		if err != nil {
			return nil, err
		}
		if len(revisions) == 0 {
			return all, nil
		}
		before = revisions[len(revisions)-1].Number
		all = append(all, revisions...)
	}
}
