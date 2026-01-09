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
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var orgName string

	cmd := &cobra.Command{
		Use:   "create <query>",
		Short: "Create a new Neo task",
		Long: "Create a new Neo task.\n" +
			"\n" +
			"This command creates a new Neo AI task with the provided query. " +
			"Neo will process your request and you can view the results in the Pulumi Cloud console.",
		Args: cmdutil.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			query := strings.Join(args, " ")

			cloudBackend, resolvedOrgName, err := requireCloudBackendWithNeo(ctx, orgName)
			if err != nil {
				return err
			}

			// Create the request with the proper message format
			req := apitype.NeoTaskRequest{
				Message: apitype.NeoMessage{
					Type:      "user_message",
					Content:   query,
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				},
			}

			resp, err := cloudBackend.CreateNeoTask(ctx, resolvedOrgName, req)
			if err != nil {
				return fmt.Errorf("failed to create Neo task: %w", err)
			}

			// Build the task URL
			cloudURL := cloudBackend.CloudURL()
			taskURL := fmt.Sprintf("%s/%s/agents/%s", cloudURL, resolvedOrgName, resp.TaskID)

			fmt.Printf("Neo task created successfully!\n\n")
			fmt.Printf("Task ID:  %s\n", resp.TaskID)
			fmt.Printf("\nView your task at:\n%s\n", taskURL)

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(
		&orgName, "org", "",
		"Organization name (defaults to current organization)",
	)

	return cmd
}
