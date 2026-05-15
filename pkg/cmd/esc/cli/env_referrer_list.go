// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvReferrerListCmd(env *envCommand) *cobra.Command {
	var (
		count                  int
		all                    bool
		allRevisions           bool
		latestStackVersionOnly bool
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
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the list command does not accept versions")
			}
			if all && count != 0 {
				return fmt.Errorf("--all and --count are mutually exclusive")
			}
			if count < 0 || count > 500 {
				return fmt.Errorf("--count must be in the range [1, 500]")
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

	return cmd
}

// printReferrers writes the response to stdout, one referrer per line, grouped by revision tag.
// "latest" is printed first, followed by numeric revision tags in ascending order, then any
// remaining non-numeric tags in lexical order.
func printReferrers(env *envCommand, resp *client.ListEnvironmentReferrersResponse) {
	if resp == nil {
		return
	}
	stdout := env.esc.stdout

	keys := sortReferrerKeys(resp.Referrers)
	for i, k := range keys {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "revision %s\n", k)
		group := resp.Referrers[k]
		// Sort referrers within a group for stable output.
		sortReferrers(group)
		for _, r := range group {
			fmt.Fprintln(stdout, formatReferrer(r))
		}
	}
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
	switch {
	case r.Environment != nil:
		return fmt.Sprintf("environment  %s/%s@%d", r.Environment.Project, r.Environment.Name, r.Environment.Revision)
	case r.Stack != nil:
		return fmt.Sprintf("stack        %s/%s@%d", r.Stack.Project, r.Stack.Stack, r.Stack.Version)
	case r.InsightsAccount != nil:
		return fmt.Sprintf("insights     %s", r.InsightsAccount.AccountName)
	default:
		return "unknown"
	}
}
