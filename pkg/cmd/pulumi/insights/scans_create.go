// Copyright 2016-2025, Pulumi Corporation.
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

package insights

import (
	"fmt"

	"github.com/spf13/cobra"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// newScansCreateCmd creates the `pulumi insights scans create` command.
func newScansCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <account-name>",
		Short: "Trigger a discovery scan",
		Long: "Trigger a discovery scan for the specified account.\n" +
			"\n" +
			"The scan will enumerate cloud resources and import them into Pulumi Insights.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			accountName := posArgs[0]

			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			fmt.Printf("Triggering scan for account %q...\n", accountName)

			scan, err := cloudBackend.Client().CreateScan(ctx, orgName, accountName, apitype.CreateScanRequest{})
			if err != nil {
				return fmt.Errorf("creating scan for account %q: %w", accountName, err)
			}

			fmt.Printf("Scan started: %s\n", scan.ID)
			fmt.Printf("Status: %s\n", scan.Status)
			fmt.Printf("\nMonitor progress:\n")
			fmt.Printf("  pulumi insights scans show %s --account %s\n", scan.ID, accountName)

			return nil
		},
	}

	return cmd
}
