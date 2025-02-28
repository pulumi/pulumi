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

package policy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newPolicyLsCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "ls [org-name]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "List all Policy Packs for a Pulumi organization",
		Long:  "List all Policy Packs for a Pulumi organization",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()

			// Try to read the current project
			ws := pkgWorkspace.Instance
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			// Get backend.
			b, err := cmdBackend.CurrentBackend(
				ctx, ws, cmdBackend.DefaultLoginManager, project,
				display.Options{Color: cmdutil.GetGlobalColorization()})
			if err != nil {
				return err
			}

			// Get organization.
			var orgName string
			if len(cliArgs) > 0 {
				orgName = cliArgs[0]
			} else {
				orgName, _, _, err = b.CurrentUser()
				if err != nil {
					return err
				}
			}

			// Gather all Policy Packs for the organization.
			var (
				allPolicyPacks []apitype.PolicyPackWithVersions
				inContToken    backend.ContinuationToken
			)
			for {
				resp, outContToken, err := b.ListPolicyPacks(ctx, orgName, inContToken)
				if err != nil {
					return err
				}

				allPolicyPacks = append(allPolicyPacks, resp.PolicyPacks...)

				if outContToken == nil {
					break
				}
				inContToken = outContToken
			}

			if jsonOut {
				return formatPolicyPacksJSON(allPolicyPacks)
			}
			return formatPolicyPacksConsole(allPolicyPacks)
		},
	}
	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	return cmd
}

func formatPolicyPacksConsole(policyPacks []apitype.PolicyPackWithVersions) error {
	// Header string and formatting options to align columns.
	headers := []string{"NAME", "VERSIONS"}

	rows := []cmdutil.TableRow{}

	for _, packs := range policyPacks {
		// Name column
		name := packs.Name

		// Version Tags column
		versionTags := strings.Trim(strings.ReplaceAll(fmt.Sprint(packs.VersionTags), " ", ", "), "[]")

		// Render the columns.
		columns := []string{name, versionTags}
		rows = append(rows, cmdutil.TableRow{Columns: columns})
	}
	ui.PrintTable(cmdutil.Table{
		Headers: headers,
		Rows:    rows,
	}, nil)
	return nil
}

// policyPacksJSON is the shape of the --json output of this command. When --json is passed, we print an array
// of policyPacksJSON objects.  While we can add fields to this structure in the future, we should not change
// existing fields.
type policyPacksJSON struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}

func formatPolicyPacksJSON(policyPacks []apitype.PolicyPackWithVersions) error {
	output := make([]policyPacksJSON, len(policyPacks))
	for i, pack := range policyPacks {
		output[i] = policyPacksJSON{
			Name:     pack.Name,
			Versions: pack.VersionTags,
		}
	}
	return ui.PrintJSON(output)
}
