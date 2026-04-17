// Copyright 2026, Pulumi Corporation.
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

package packagecmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newPackageLsCmd() *cobra.Command {
	var orgName string
	var pkgType string
	var limit int

	cmd := &cobra.Command{
		Use:   "ls [query]",
		Short: "List packages in the registry",
		Long: `List packages in the registry.

Without a query, lists all packages. With a query, searches by name.
Use --org to scope results to an organization's packages.

If --org is not specified, the default organization is resolved from
the Pulumi configuration, then from the backend.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := validateLimit(limit); err != nil {
				return err
			}

			if orgName == "" {
				orgName = resolveDefaultOrg(ctx)
			}

			var search string
			if len(args) > 0 {
				search = strings.Join(args, " ")
			}

			sort, asc := defaultSort(orgName, search)

			reg := registryForContext(ctx)
			packages, err := reg.SearchPackages(ctx, apitype.PackageSearchOptions{
				OrgName: orgName,
				Search:  search,
				Type:    pkgType,
				Sort:    sort,
				Asc:     asc,
				Limit:   limit,
			})
			if err != nil {
				return err
			}

			if len(packages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No packages found.")
				return nil
			}

			rows := make([]cmdutil.TableRow, len(packages))
			for i, pkg := range packages {
				registryURL := fmt.Sprintf("%s/%s/%s", pkg.Source, pkg.Publisher, pkg.Name)
				pkgTypes := ""
				if len(pkg.PackageTypes) > 0 {
					pkgTypes = string(pkg.PackageTypes[0])
				}
				rows[i] = cmdutil.TableRow{
					Columns: []string{pkg.Name, pkg.Version.String(), registryURL, pkgTypes},
				}
			}

			ui.PrintTable(cmdutil.Table{
				Headers: []string{"NAME", "VERSION", "REGISTRY URL", "TYPE"},
				Rows:    rows,
			}, nil)

			return nil
		},
	}

	cmd.Flags().StringVar(&orgName, "org", "", "Organization to list packages for, including private packages (defaults to configured default org)")
	cmd.Flags().StringVar(&pkgType, "type", "", "Filter by package type (provider, component)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results (max 500)")

	return cmd
}

// defaultSort returns sort field and direction based on context, matching
// the console UI behavior: usageTotal desc when browsing an org without a
// search query, name asc otherwise.
func defaultSort(orgName, search string) (string, *bool) {
	if orgName != "" && search == "" {
		asc := false
		return "usageTotal", &asc
	}
	asc := true
	return "name", &asc
}

// resolveDefaultOrg attempts to resolve the default organization
// non-interactively. Returns empty string if not logged in or no default set.
func resolveDefaultOrg(ctx context.Context) string {
	project, _, err := pkgWorkspace.Instance.ReadProject()
	if err != nil {
		project = nil
	}
	b, err := cmdBackend.NonInteractiveCurrentBackend(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, project,
	)
	if err != nil || b == nil {
		return ""
	}
	org, err := backend.GetDefaultOrg(ctx, b, project)
	if err != nil {
		return ""
	}
	return org
}
