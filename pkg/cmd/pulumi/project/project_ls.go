// Copyright 2016-2024, Pulumi Corporation.
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

package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdbackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// projectListResult represents the results of a project listing for display purposes.
type projectListResult struct {
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	StackCount   int    `json:"stackCount,omitempty"`
}

func newProjectLsCmd() *cobra.Command {
	var orgName string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List your Pulumi projects",
		Long: "List your Pulumi projects.\n" +
			"\n" +
			"This command lists all Pulumi projects accessible to the current user.",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Get the current workspace and project if any.
			ws := pkgWorkspace.Instance
			var err error
			var currentProject *workspace.Project
			currentProject, _, err = ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			// Get the current backend.
			b, err := cmdbackend.CurrentBackend(ctx, ws, cmdbackend.DefaultLoginManager, currentProject, displayOpts)
			if err != nil {
				return err
			}

			// Handle DIY backends - they don't support organizations directly
			if !b.SupportsOrganizations() {
				// If user specifically requested organization info but we can't provide it
				if orgName != "" {
					return fmt.Errorf("organizations are not supported by the current backend (%s)", b.Name())
				}

				// For DIY backends, list all local projects by stack names
				stackSummaries, _, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil)
				if err != nil {
					return err
				}

				// Map to track projects and stack counts (in DIY mode)
				projectMap := make(map[string]projectListResult)

				// Group stacks by project
				for _, stack := range stackSummaries {
					// Get project name from stack reference
					projectName, ok := stack.Name().Project()
					if !ok {
						// Skip stacks without a project
						continue
					}

					// Use the string representation of projectName
					projectNameStr := string(projectName)

					// Update project entry
					project, exists := projectMap[projectNameStr]
					if !exists {
						project = projectListResult{
							Name:       projectNameStr,
							StackCount: 0,
						}
					}
					project.StackCount++
					projectMap[projectNameStr] = project
				}

				// Convert map to slice for output
				var results []projectListResult
				for _, project := range projectMap {
					results = append(results, project)
				}

				// If no projects, display a message
				if len(results) == 0 {
					fmt.Println("No projects found")
					return nil
				}

				// Output the results
				if jsonOut {
					out, err := json.MarshalIndent(results, "", "    ")
					if err != nil {
						return err
					}
					fmt.Println(string(out))
				} else {
					fmt.Println("PROJECTS (local):")
					for _, result := range results {
						fmt.Printf("  %s (stacks: %d)\n", result.Name, result.StackCount)
					}
				}

				return nil
			}

			// Get the current user.
			_, userOrgs, _, err := b.CurrentUser()
			if err != nil {
				return err
			}

			// If org name is specified, verify the user has access to it.
			if orgName != "" {
				// Verify the organization exists and the user has access to it.
				hasAccess := false
				for _, org := range userOrgs {
					if org == orgName {
						hasAccess = true
						break
					}
				}
				if !hasAccess {
					return fmt.Errorf("you do not have access to the organization %s", orgName)
				}
			}

			// Create a filter for ListStacks
			filter := backend.ListStacksFilter{}
			if orgName != "" {
				filter.Organization = &orgName
			}

			// List all stacks and group them by project
			stackSummaries, _, err := b.ListStacks(ctx, filter, nil)
			if err != nil {
				return err
			}

			// Map to track projects and stack counts
			projectMap := make(map[string]projectListResult)

			// Group stacks by project
			for _, stack := range stackSummaries {
				// Get project name from stack reference
				projectName, ok := stack.Name().Project()
				if !ok {
					// Skip stacks without a project
					continue
				}

				// Use the string representation of projectName
				projectNameStr := string(projectName)

				// Get organization from the fully qualified name
				// Format is typically org/project/stack
				stackFullName := stack.Name().String()
				stackOrg := getOrgFromStackName(stackFullName)

				// Skip if we're filtering by organization and this stack doesn't match
				if orgName != "" && stackOrg != orgName {
					continue
				}

				// Update project entry
				project, exists := projectMap[projectNameStr]
				if !exists {
					project = projectListResult{
						Name:         projectNameStr,
						Organization: stackOrg,
						StackCount:   0,
					}
				}
				project.StackCount++
				projectMap[projectNameStr] = project
			}

			// Convert map to slice for output
			var results []projectListResult
			for _, project := range projectMap {
				results = append(results, project)
			}

			// If no projects, display a message.
			if len(results) == 0 {
				if orgName != "" {
					fmt.Println("No projects found in organization", orgName)
				} else {
					fmt.Println("No projects found")
				}
				return nil
			}

			// Output the results.
			if jsonOut {
				out, err := json.MarshalIndent(results, "", "    ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
			} else {
				// Print a nice formatted table.
				if orgName != "" {
					fmt.Printf("PROJECTS IN ORGANIZATION %s:\n", orgName)
				} else {
					fmt.Println("PROJECTS:")
				}

				for _, result := range results {
					if orgName == "" {
						fmt.Printf("  %s (org: %s, stacks: %d)\n", result.Name, result.Organization, result.StackCount)
					} else {
						fmt.Printf("  %s (stacks: %d)\n", result.Name, result.StackCount)
					}
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&orgName, "organization", "o", "", "The organization whose projects to list")
	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

// getOrgFromStackName extracts the organization name from a fully qualified stack name.
// Stack names are typically in the format "org/project/stack" or just "project/stack".
func getOrgFromStackName(stackName string) string {
	parts := strings.Split(stackName, "/")
	if len(parts) < 3 {
		return "" // No organization in the stack name
	}
	return parts[0]
}
