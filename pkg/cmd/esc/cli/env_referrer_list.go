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
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvReferrerListCmd(env *envCommand) *cobra.Command {
	var (
		count                  int
		all                    bool
		allRevisions           bool
		latestStackVersionOnly bool
		output                 string
	)

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name>",
		Aliases: []string{"ls"},
		Short:   "List entities that reference an environment.",
		Long: "[EXPERIMENTAL] List entities that reference an environment\n" +
			"\n" +
			"This command lists referrers (other environments, Pulumi IaC stacks, and Pulumi\n" +
			"Insights accounts) that reference the given environment. Results are grouped by\n" +
			"the revision of the referenced environment.\n",
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
				return errors.New("the list command does not accept versions")
			}
			if all && count != 0 {
				return errors.New("--all and --count are mutually exclusive")
			}
			if count < 0 || count > 500 {
				return errors.New("--count must be in the range [1, 500]")
			}

			merged := &client.ListEnvironmentReferrersResponse{
				Referrers: map[string][]client.EnvironmentReferrer{},
			}
			remaining := count
			continuationToken := ""
			for {
				opts := client.ListEnvironmentReferrersOptions{
					ContinuationToken: continuationToken,
				}
				if remaining > 0 {
					opts.Count = &remaining
				}
				if allRevisions {
					opts.AllRevisions = &allRevisions
				}
				if latestStackVersionOnly {
					opts.LatestStackVersionOnly = &latestStackVersionOnly
				}

				resp, err := env.esc.client.ListEnvironmentReferrers(ctx, ref.orgName, ref.projectName, ref.envName, opts)
				if err != nil {
					return err
				}
				for k, v := range resp.Referrers {
					merged.Referrers[k] = append(merged.Referrers[k], v...)
					if count > 0 {
						remaining -= len(v)
					}
				}
				if !all || resp.ContinuationToken == "" {
					break
				}
				if count > 0 && remaining <= 0 {
					break
				}
				continuationToken = resp.ContinuationToken
			}

			if format == outputJSON {
				return writeJSON(env.esc.stdout, struct {
					Referrers map[string][]client.EnvironmentReferrer `json:"referrers"`
				}{merged.Referrers})
			}

			printReferrers(env, merged)
			return nil
		},
	}

	cmd.Flags().IntVar(&count, "count", 0,
		"the maximum number of referrers to return (server default if unset; max 500). Mutually exclusive with --all")
	cmd.Flags().BoolVar(&all, "all", false,
		"return all referrers, paginating through every page. Mutually exclusive with --count")
	cmd.Flags().BoolVar(&allRevisions, "all-revisions", false,
		"include referrers across all revisions of the environment, not just the latest")
	cmd.Flags().BoolVar(&latestStackVersionOnly, "latest-stack-version-only", false,
		"only include the latest version of each referring stack")
	addOutputFlag(cmd, &output)

	return cmd
}

// printReferrers writes the response as a single table with a REVISION column.
// Rows are ordered by revision: "latest" first, then numeric tags ascending, then
// remaining non-numeric tags in lexical order. Within a revision, rows are sorted
// for stable output.
func printReferrers(env *envCommand, resp *client.ListEnvironmentReferrersResponse) {
	if resp == nil || len(resp.Referrers) == 0 {
		return
	}

	t := newTable(env.esc.stdout)
	t.AppendHeader(table.Row{"REVISION", "KIND", "REFERRER"})
	for _, k := range sortReferrerKeys(resp.Referrers) {
		group := resp.Referrers[k]
		sortReferrers(group)
		for _, r := range group {
			kind, ref := referrerColumns(r)
			t.AppendRow(table.Row{k, kind, ref})
		}
	}
	t.Render()
}

func sortReferrerKeys(m map[string][]client.EnvironmentReferrer) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		// "latest" sorts first.
		if ki == "latest" {
			return kj != "latest"
		}
		if kj == "latest" {
			return false
		}
		// Numeric tags sort before non-numeric.
		ni, ei := strconv.Atoi(ki)
		nj, ej := strconv.Atoi(kj)
		switch {
		case ei == nil && ej == nil:
			return ni < nj
		case ei == nil:
			return true
		case ej == nil:
			return false
		default:
			return ki < kj
		}
	})
	return keys
}

func sortReferrers(rs []client.EnvironmentReferrer) {
	sort.SliceStable(rs, func(i, j int) bool {
		return formatReferrer(rs[i]) < formatReferrer(rs[j])
	})
}

func formatReferrer(r client.EnvironmentReferrer) string {
	kind, ref := referrerColumns(r)
	return fmt.Sprintf("%s %s", kind, ref)
}

func referrerColumns(r client.EnvironmentReferrer) (kind, ref string) {
	switch {
	case r.Environment != nil:
		return "environment", fmt.Sprintf("%s/%s@%d", r.Environment.Project, r.Environment.Name, r.Environment.Revision)
	case r.Stack != nil:
		return "stack", fmt.Sprintf("%s/%s@%d", r.Stack.Project, r.Stack.Stack, r.Stack.Version)
	case r.InsightsAccount != nil:
		return "insights", r.InsightsAccount.AccountName
	default:
		return "unknown", ""
	}
}
