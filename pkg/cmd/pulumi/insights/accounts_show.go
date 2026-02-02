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
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type accountsShowArgs struct {
	output outputFormat
}

// newAccountsShowCmd creates the `pulumi insights accounts show` command.
func newAccountsShowCmd() *cobra.Command {
	args := &accountsShowArgs{
		output: outputFormatTable,
	}

	cmd := &cobra.Command{
		Use:   "show <account-name>",
		Short: "Show Insights discovery account details",
		Long:  "Show detailed information about an Insights discovery account.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			accountName := posArgs[0]

			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			account, err := cloudBackend.Client().GetInsightsAccount(ctx, orgName, accountName)
			if err != nil {
				return fmt.Errorf("getting account %q: %w", accountName, err)
			}

			switch args.output {
			case outputFormatJSON:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(account)
			case outputFormatYAML:
				return yaml.NewEncoder(os.Stdout).Encode(account)
			default:
				return renderAccountDetail(account)
			}
		},
	}

	cmd.Flags().VarP(&args.output, "output", "o", `output format: "table", "json", or "yaml"`)

	return cmd
}

// renderAccountDetail renders a detailed view of an Insights account.
func renderAccountDetail(account *apitype.InsightsAccount) error {
	fmt.Printf("Account: %s\n", account.Name)
	fmt.Printf("Provider: %s\n", strings.ToUpper(account.Provider))
	fmt.Printf("ESC Environment: %s\n", account.Environment)

	if account.ProviderConfig != nil {
		if regions, ok := account.ProviderConfig["regions"]; ok {
			fmt.Printf("Regions: %v\n", regions)
		}
	}

	if account.ScanSchedule != "" {
		fmt.Printf("Scan Schedule: %s\n", account.ScanSchedule)
	}

	if account.ScanStatus != nil {
		fmt.Printf("\nLast Scan:\n")
		fmt.Printf("  Status: %s\n", account.ScanStatus.Status)
		if account.ScanStatus.StartedAt != nil {
			fmt.Printf("  Started: %s (%s)\n",
				account.ScanStatus.StartedAt.Format("2006-01-02 15:04:05 UTC"),
				formatTimeAgo(*account.ScanStatus.StartedAt))
		}
		if account.ScanStatus.FinishedAt != nil {
			fmt.Printf("  Finished: %s (%s)\n",
				account.ScanStatus.FinishedAt.Format("2006-01-02 15:04:05 UTC"),
				formatTimeAgo(*account.ScanStatus.FinishedAt))
		}
		if account.ScanStatus.ResourceCount > 0 {
			fmt.Printf("  Resources: %d\n", account.ScanStatus.ResourceCount)
		}
		if account.ScanStatus.NextScheduledScan != nil {
			fmt.Printf("  Next Scheduled Scan: %s\n",
				account.ScanStatus.NextScheduledScan.Format("2006-01-02 15:04:05 UTC"))
		}
	}

	if len(account.ChildAccounts) > 0 {
		fmt.Printf("\nChild Accounts:\n")
		for _, child := range account.ChildAccounts {
			fmt.Printf("  - %s\n", child)
		}
	}

	return nil
}
