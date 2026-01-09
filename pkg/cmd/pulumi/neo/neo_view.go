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

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newViewCmd() *cobra.Command {
	var orgName string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "view [task-id]",
		Short: "View details of a Neo task",
		Long: "View details of a Neo task.\n" +
			"\n" +
			"This command displays detailed information about a specific Neo task.\n" +
			"If no task ID is provided, you will be prompted to select from the 50 most recent tasks.",
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cloudBackend, resolvedOrgName, err := requireCloudBackendWithNeo(ctx, orgName)
			if err != nil {
				return err
			}

			var taskID string
			if len(args) == 0 {
				// No task ID provided, prompt user to select from recent tasks
				// Fetch 50 most recent tasks
				resp, err := cloudBackend.ListNeoTasks(ctx, resolvedOrgName, 50, "")
				if err != nil {
					return fmt.Errorf("failed to list Neo tasks: %w", err)
				}

				if len(resp.Tasks) == 0 {
					return fmt.Errorf("no Neo tasks found for organization %s", resolvedOrgName)
				}

				tasks := resp.Tasks

				// Build options for the prompt
				options := make([]string, len(tasks))
				taskMap := make(map[string]string) // maps display string to task ID
				for i, task := range tasks {
					// Format: "task-name (task-id) - status - created"
					// Safely truncate ID to 8 chars if it's long enough
					idDisplay := task.ID
					if len(idDisplay) > 8 {
						idDisplay = idDisplay[:8]
					}
					displayStr := fmt.Sprintf("%s (%s) - %s - %s", task.Name, idDisplay, task.Status, task.CreatedAt)
					options[i] = displayStr
					taskMap[displayStr] = task.ID
				}

				selected := ui.PromptUser(
					"Select a Neo task:",
					options,
					options[0],
					cmdutil.GetGlobalColorization(),
				)

				if selected == "" {
					return fmt.Errorf("no task selected")
				}

				taskID = taskMap[selected]
			} else {
				taskID = args[0]
			}

			task, err := cloudBackend.GetNeoTask(ctx, resolvedOrgName, taskID)
			if err != nil {
				return fmt.Errorf("failed to get Neo task: %w", err)
			}

			if jsonOut {
				return ui.PrintJSON(task)
			}

			fmt.Printf("Task ID:   %s\n", task.ID)
			fmt.Printf("Name:      %s\n", task.Name)
			fmt.Printf("Status:    %s\n", task.Status)
			fmt.Printf("Created:   %s\n", task.CreatedAt)

			if len(task.Entities) > 0 {
				fmt.Printf("\nEntities:\n")
				for _, entity := range task.Entities {
					fmt.Printf("  - %s: %s", entity.Type, entity.Name)
					if entity.Project != "" {
						fmt.Printf(" (project: %s)", entity.Project)
					}
					fmt.Println()
				}
			}

			// Build the task URL
			cloudURL := cloudBackend.CloudURL()
			taskURL := fmt.Sprintf("%s/%s/agents/%s", cloudURL, resolvedOrgName, taskID)
			fmt.Printf("\nView in console:\n%s\n", taskURL)

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

	return cmd
}
