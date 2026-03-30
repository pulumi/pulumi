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

package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blang/semver"
	cmdcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemarender"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type functionListItemJSON struct {
	Token       string `json:"token"`
	Module      string `json:"module"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func newRegistryFunctionLsCmd() *cobra.Command {
	var jsonOut bool
	var tokensOnly bool
	var module string
	var versionStr string

	cmd := &cobra.Command{
		Use:   "ls <package>",
		Short: "List functions in a package",
		Long: `List all functions (data sources / invokes) defined by a package
in the Pulumi Registry.

Use --module to filter by module name.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			reg := cmdcmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())

			var version *semver.Version
			if versionStr != "" {
				v, err := semver.Parse(versionStr)
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", versionStr, err)
				}
				version = &v
			}

			spec, err := loadSchemaForPackage(ctx, reg, args[0], version)
			if err != nil {
				return err
			}

			items, err := collectFunctions(spec, module)
			if err != nil {
				return err
			}

			if len(items) == 0 {
				if module != "" {
					return fmt.Errorf("no functions found in module %q", module)
				}
				fmt.Println("No functions found")
				return nil
			}

			if jsonOut {
				return ui.PrintJSON(items)
			}
			if tokensOnly {
				for _, item := range items {
					fmt.Println(item.Token)
				}
				return nil
			}

			if cmdutil.Interactive() {
				listItems := make([]registryItem, len(items))
				for i, item := range items {
					parts := strings.Split(item.Token, ":")
					listItems[i] = registryItem{
						title:      fmt.Sprintf("%-12s %-40s %s", parts[0], parts[1], parts[2]),
						filterText: parts[1] + " " + parts[2],
						value:      i,
					}
				}
				idx, err := runInteractiveList(
					fmt.Sprintf("%d functions in %s", len(items), args[0]),
					fmt.Sprintf("%-12s %-40s %s", "PACKAGE", "MODULE", "FUNCTION"),
					listItems)
				if err != nil {
					return err
				}
				if idx >= 0 {
					fmt.Println()
					token := items[idx].Token
					resolvedToken, fn, err := findFunction(spec, token)
					if err != nil {
						return err
					}
					return formatFunctionDetailConsole(spec, resolvedToken, fn, true)
				}
				return nil
			}

			return formatFunctionListConsole(items)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().BoolVar(&tokensOnly, "tokens", false, "Output raw tokens only, one per line")
	cmd.PersistentFlags().StringVarP(&module, "module", "m", "", "Filter by module name")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific package version")

	return cmd
}

func collectFunctions(spec *schema.PackageSpec, moduleFilter string) ([]functionListItemJSON, error) {
	var items []functionListItemJSON
	for token, fn := range spec.Functions {
		parts := strings.Split(token, ":")
		if len(parts) < 3 {
			continue
		}
		moduleName := strings.Split(parts[1], "/")[0]

		if moduleFilter != "" && moduleName != moduleFilter && parts[1] != moduleFilter {
			continue
		}

		items = append(items, functionListItemJSON{
			Token:       token,
			Module:      moduleName,
			Name:        parts[2],
			Description: schemarender.SummaryFromDescription(fn.Description),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Token < items[j].Token
	})

	return items, nil
}

func formatFunctionListConsole(items []functionListItemJSON) error {
	rows := make([]cmdutil.TableRow, len(items))
	for i, item := range items {
		parts := strings.Split(item.Token, ":")
		pkg := parts[0]
		module := parts[1]
		typeName := parts[2]
		rows[i] = cmdutil.TableRow{
			Columns: []string{pkg, module, typeName},
		}
	}

	ui.PrintTable(cmdutil.Table{
		Headers: []string{"PACKAGE", "MODULE", "FUNCTION"},
		Rows:    rows,
	}, nil)

	fmt.Printf("\nTotal: %d functions\n", len(items))
	return nil
}
