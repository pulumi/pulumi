// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvLsCmd(env *envCommand) *cobra.Command {
	var (
		orgFilter     string
		projectFilter string
	)

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List environments.",
		Long: "List environments\n" +
			"\n" +
			"This command lists environments. All environments you have access to will be listed.\n",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			allEnvs, err := env.listEnvironments(ctx, orgFilter, projectFilter)
			if err != nil {
				return err
			}

			sort.Slice(allEnvs, func(i, j int) bool {
				if allEnvs[i].Organization == allEnvs[j].Organization {
					return allEnvs[i].Name < allEnvs[j].Name
				}
				return allEnvs[i].Organization < allEnvs[j].Organization
			})

			for _, e := range allEnvs {
				if e.Organization == "" {
					fmt.Fprintf(env.esc.stdout, "%v/%v\n", e.Project, e.Name)
				} else {
					fmt.Fprintf(env.esc.stdout, "%v/%v/%v\n", e.Organization, e.Project, e.Name)
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&orgFilter, "organization", "o", "", "Filter returned environments to those in a specific organization")
	cmd.PersistentFlags().StringVarP(
		&projectFilter, "project", "p", "", "Filter returned environments to those in a specific project")

	return cmd
}

func (env *envCommand) listEnvironments(ctx context.Context, orgFilter, projectFilter string) ([]client.OrgEnvironment, error) {
	user := env.esc.account.Username
	continuationToken, allEnvs := "", []client.OrgEnvironment(nil)
	for {
		envs, nextToken, err := env.esc.client.ListEnvironments(ctx, orgFilter, continuationToken)
		if err != nil {
			return []client.OrgEnvironment(nil), fmt.Errorf("listing environments: %w", err)
		}
		for _, e := range envs {
			if e.Organization == user {
				e.Organization = ""
			}
			if projectFilter != "" && e.Project != projectFilter {
				continue
			}
			allEnvs = append(allEnvs, e)
		}
		if nextToken == "" {
			break
		}
		continuationToken = nextToken
	}

	return allEnvs, nil
}
