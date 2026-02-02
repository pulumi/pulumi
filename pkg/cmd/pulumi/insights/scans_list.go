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
	auto_table "go.pennock.tech/tabular/auto"
	"gopkg.in/yaml.v3"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type scansListArgs struct {
	output outputFormat
	limit  int
}

// newScansListCmd creates the `pulumi insights scans list` command.
func newScansListCmd() *cobra.Command {
	args := &scansListArgs{
		output: outputFormatTable,
		limit:  20,
	}

	cmd := &cobra.Command{
		Use:   "list <account-name>",
		Short: "List discovery scans",
		Long:  "List discovery scans for the specified account.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			accountName := posArgs[0]

			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			resp, err := cloudBackend.Client().ListScans(ctx, orgName, accountName, "", args.limit)
			if err != nil {
				return fmt.Errorf("listing scans for account %q: %w", accountName, err)
			}

			if len(resp.Scans) == 0 {
				fmt.Printf("No scans found for account %q.\n", accountName)
				fmt.Printf("\nTrigger a scan: pulumi insights scans create %s\n", accountName)
				return nil
			}

			switch args.output {
			case outputFormatJSON:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(resp.Scans)
			case outputFormatYAML:
				return yaml.NewEncoder(os.Stdout).Encode(resp.Scans)
			default:
				return renderScansTable(os.Stdout, resp.Scans)
			}
		},
	}

	cmd.Flags().VarP(&args.output, "output", "o", `output format: "table", "json", or "yaml"`)
	cmd.Flags().IntVar(&args.limit, "limit", 20, "maximum number of scans to list")

	return cmd
}

// renderScansTable renders a table of scans.
func renderScansTable(w *os.File, scans []apitype.Scan) error {
	table := auto_table.New("utf8-heavy")
	table.AddHeaders("SCAN ID", "ACCOUNT", "STATUS", "STARTED", "RESOURCES")

	for _, s := range scans {
		started := "-"
		if s.StartedAt != nil {
			started = formatTimeAgo(*s.StartedAt)
		}

		resources := "-"
		if s.ResourceCount > 0 {
			resources = fmt.Sprintf("%d", s.ResourceCount)
		}

		table.AddRowItems(
			s.ID,
			s.AccountName,
			string(s.Status),
			started,
			resources,
		)
	}

	return table.RenderTo(w)
}
