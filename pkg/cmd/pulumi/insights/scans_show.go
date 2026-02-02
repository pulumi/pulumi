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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

type scansShowArgs struct {
	account string
	output  outputFormat
}

// newScansShowCmd creates the `pulumi insights scans show` command.
func newScansShowCmd() *cobra.Command {
	args := &scansShowArgs{
		output: outputFormatTable,
	}

	cmd := &cobra.Command{
		Use:   "show <scan-id>",
		Short: "Show discovery scan details",
		Long:  "Show detailed information about a discovery scan.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			scanID := posArgs[0]

			if args.account == "" {
				return fmt.Errorf("--account flag is required")
			}

			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			scan, err := cloudBackend.Client().GetScan(ctx, orgName, args.account, scanID)
			if err != nil {
				return fmt.Errorf("getting scan %q: %w", scanID, err)
			}

			switch args.output {
			case outputFormatJSON:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(scan)
			case outputFormatYAML:
				return yaml.NewEncoder(os.Stdout).Encode(scan)
			default:
				fmt.Printf("Scan: %s\n", scan.ID)
				fmt.Printf("Account: %s\n", scan.AccountName)
				fmt.Printf("Status: %s\n", scan.Status)
				if scan.StartedAt != nil {
					fmt.Printf("Started: %s (%s)\n",
						scan.StartedAt.Format("2006-01-02 15:04:05 UTC"),
						formatTimeAgo(*scan.StartedAt))
				}
				if scan.CompletedAt != nil {
					fmt.Printf("Completed: %s (%s)\n",
						scan.CompletedAt.Format("2006-01-02 15:04:05 UTC"),
						formatTimeAgo(*scan.CompletedAt))
				}
				if scan.ResourceCount > 0 {
					fmt.Printf("Resources: %d\n", scan.ResourceCount)
				}
				if scan.ErrorCount > 0 {
					fmt.Printf("Errors: %d\n", scan.ErrorCount)
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&args.account, "account", "", "account name (required)")
	cmd.Flags().VarP(&args.output, "output", "o", `output format: "table", "json", or "yaml"`)

	return cmd
}
