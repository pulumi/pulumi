// Copyright 2025, Pulumi Corporation.
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

package neo

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var orgName string
	var jsonOut bool
	var statusFilter string
	var pageSize int
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Neo tasks for an organization",
		Long: "List Neo tasks for an organization.\n" +
			"\n" +
			"This command lists Neo tasks for the specified organization.\n" +
			"By default, returns up to 100 tasks. Use --all to fetch all tasks.\n" +
			"Use --status to filter by task status (running or idle).",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cloudBackend, resolvedOrgName, err := requireCloudBackendWithNeo(ctx, orgName)
			if err != nil {
				return err
			}

			// Validate status filter
			if statusFilter != "" {
				statusFilter = strings.ToLower(statusFilter)
				if statusFilter != "running" && statusFilter != "idle" {
					return fmt.Errorf("invalid status filter %q: must be 'running' or 'idle'", statusFilter)
				}
			}

			// Collect all tasks (with pagination if --all is specified)
			var allTasks []apitype.NeoTask
			continuationToken := ""
			requestPageSize := pageSize
			if requestPageSize <= 0 {
				requestPageSize = 100
			}

			for {
				resp, err := cloudBackend.ListNeoTasks(ctx, resolvedOrgName, requestPageSize, continuationToken)
				if err != nil {
					return fmt.Errorf("failed to list Neo tasks: %w", err)
				}

				// Apply status filter
				for _, task := range resp.Tasks {
					if statusFilter == "" || strings.ToLower(task.Status) == statusFilter {
						allTasks = append(allTasks, task)
					}
				}

				// Check if we should continue paginating
				if !all || resp.ContinuationToken == "" {
					break
				}
				continuationToken = resp.ContinuationToken
			}

			if jsonOut {
				return ui.PrintJSON(allTasks)
			}

			if len(allTasks) == 0 {
				if statusFilter != "" {
					fmt.Printf("No %s Neo tasks found for organization %s\n", statusFilter, resolvedOrgName)
				} else {
					fmt.Printf("No Neo tasks found for organization %s\n", resolvedOrgName)
				}
				return nil
			}

			headers := []string{"ID", "STATUS", "NAME", "CREATED"}
			rows := []cmdutil.TableRow{}

			for _, task := range allTasks {
				// Truncate name if too long
				name := task.Name
				if len(name) > 60 {
					name = name[:57] + "..."
				}

				columns := []string{
					task.ID,
					task.Status,
					name,
					task.CreatedAt,
				}
				rows = append(rows, cmdutil.TableRow{Columns: columns})
			}

			ui.PrintTable(cmdutil.Table{
				Headers: headers,
				Rows:    rows,
			}, nil)

			if !all && len(allTasks) == requestPageSize {
				fmt.Printf("\nShowing first %d tasks. Use --all to fetch all tasks.\n", requestPageSize)
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(
		&orgName, "org", "",
		"Organization name (defaults to current organization)",
	)
	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false,
		"Emit output as JSON",
	)
	cmd.PersistentFlags().StringVar(
		&statusFilter, "status", "",
		"Filter by task status (running or idle)",
	)
	cmd.PersistentFlags().IntVar(
		&pageSize, "page-size", 100,
		"Number of tasks to fetch per page (1-1000, default 100)",
	)
	cmd.PersistentFlags().BoolVar(
		&all, "all", false,
		"Fetch all tasks (automatically handles pagination)",
	)

	return cmd
}
