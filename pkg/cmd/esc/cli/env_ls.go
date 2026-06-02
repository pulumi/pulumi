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
	"sort"

	"github.com/spf13/cobra"

	client "github.com/pulumi/pulumi/sdk/v3/go/esc/cloud"
)

func newEnvLsCmd(env *envCommand) *cobra.Command {
	var (
		orgFilter     string
		projectFilter string
		output        string
	)

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List environments.",
		Long: "List environments\n" +
			"\n" +
			"This command lists environments. All environments you have access to will be listed.\n",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			allEnvs, err := env.listEnvironments(ctx, orgFilter, projectFilter)
			if err != nil {
				return err
			}

			sort.Slice(allEnvs, func(i, j int) bool {
				ei, ej := allEnvs[i], allEnvs[j]

				if ei.Organization == ej.Organization {
					if ei.Project == ej.Project {
						return ei.Name < ej.Name
					}
					return ei.Project < ej.Project
				}
				return ei.Organization < ej.Organization
			})

			if format == outputJSON {
				ids := make([]string, 0, len(allEnvs))
				for _, e := range allEnvs {
					ids = append(ids, envIdentifier(e))
				}
				return writeJSON(env.esc.stdout, struct {
					Environments []string `json:"environments"`
				}{ids})
			}

			for _, e := range allEnvs {
				fmt.Fprintln(env.esc.stdout, envIdentifier(e))
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&orgFilter, "organization", "o", "", "Filter returned environments to those in a specific organization")
	cmd.PersistentFlags().StringVarP(
		&projectFilter, "project", "p", "", "Filter returned environments to those in a specific project")
	addOutputFlag(cmd, &output)

	return cmd
}

// envIdentifier formats an environment as "org/project/name", omitting the org segment
// when it has been blanked out (i.e. it matches the caller's own username).
func envIdentifier(e client.OrgEnvironment) string {
	if e.Organization == "" {
		return fmt.Sprintf("%s/%s", e.Project, e.Name)
	}
	return fmt.Sprintf("%s/%s/%s", e.Organization, e.Project, e.Name)
}

func (env *envCommand) listEnvironments(ctx context.Context, orgFilter, projectFilter string) ([]client.OrgEnvironment, error) {
	user := env.esc.account.Username
	continuationToken, allEnvs := "", []client.OrgEnvironment(nil)
	for {
		var envs []client.OrgEnvironment
		var nextToken string
		var err error

		// If orgFilter is specified, use ListOrganizationEnvironments endpoint, so that we receive proper errors
		// like 404 when environment doesn't exist, instead of an empty array
		if orgFilter != "" {
			envs, nextToken, err = env.esc.client.ListOrganizationEnvironments(ctx, orgFilter, continuationToken)
		} else {
			envs, nextToken, err = env.esc.client.ListEnvironments(ctx, continuationToken)
		}

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
