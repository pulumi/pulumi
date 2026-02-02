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
	"time"

	"github.com/spf13/cobra"
	auto_table "go.pennock.tech/tabular/auto"
	"gopkg.in/yaml.v3"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type accountsListArgs struct {
	output outputFormat
}

// newAccountsListCmd creates the `pulumi insights accounts list` command.
func newAccountsListCmd() *cobra.Command {
	args := &accountsListArgs{
		output: outputFormatTable,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Insights discovery accounts",
		Long: "List Insights discovery accounts.\n" +
			"\n" +
			"Lists all discovery accounts including parent and child (per-region) accounts.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance

			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			// Fetch all accounts with pagination
			var allAccounts []apitype.InsightsAccount
			continuationToken := ""
			for {
				resp, listErr := cloudBackend.Client().ListInsightsAccounts(ctx, orgName, continuationToken, 100)
				if listErr != nil {
					return fmt.Errorf("listing accounts: %w", listErr)
				}
				allAccounts = append(allAccounts, resp.Accounts...)
				if resp.ContinuationToken == "" {
					break
				}
				continuationToken = resp.ContinuationToken
			}

			if len(allAccounts) == 0 {
				fmt.Println("No discovery accounts found.")
				fmt.Println("\nCreate one with: pulumi insights accounts create")
				return nil
			}

			switch args.output {
			case outputFormatTable:
				return renderAccountsTable(os.Stdout, allAccounts)
			case outputFormatJSON:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(allAccounts)
			case outputFormatYAML:
				return yaml.NewEncoder(os.Stdout).Encode(allAccounts)
			default:
				return renderAccountsTable(os.Stdout, allAccounts)
			}
		},
	}

	cmd.Flags().VarP(&args.output, "output", "o", `output format: "table", "json", or "yaml"`)

	return cmd
}

// renderAccountsTable renders a table of Insights accounts.
func renderAccountsTable(w *os.File, accounts []apitype.InsightsAccount) error {
	table := auto_table.New("utf8-heavy")
	table.AddHeaders("NAME", "PROVIDER", "STATUS", "LAST SCAN", "RESOURCES")

	for _, acct := range accounts {
		status := "Active"
		lastScan := "Never"
		resources := "-"

		if acct.ScanStatus != nil {
			status = string(acct.ScanStatus.Status)
			if acct.ScanStatus.StartedAt != nil {
				lastScan = formatTimeAgo(*acct.ScanStatus.StartedAt)
			}
			if acct.ScanStatus.ResourceCount > 0 {
				resources = fmt.Sprintf("%d", acct.ScanStatus.ResourceCount)
			}
		}

		table.AddRowItems(
			acct.Name,
			strings.ToUpper(acct.Provider),
			status,
			lastScan,
			resources,
		)
	}

	return table.RenderTo(w)
}

// formatTimeAgo formats a time as a human-readable relative string.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
