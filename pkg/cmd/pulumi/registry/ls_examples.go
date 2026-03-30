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
	"strings"

	"github.com/blang/semver"
	cmdcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type exampleListItemJSON struct {
	Index     int      `json:"index"`
	Title     string   `json:"title"`
	Languages []string `json:"languages"`
}

func newRegistryExampleLsCmd() *cobra.Command {
	var jsonOut bool
	var versionStr string
	var limit int

	cmd := &cobra.Command{
		Use:   "ls <package-or-token>",
		Short: "List code examples",
		Long: `List available code examples for a package, resource, or function.

Pass a package name (e.g., aws) for package-level examples, or a type token
(e.g., aws:ec2/instance:Instance) for resource or function examples.

Use --limit to cap the number of results shown (default 50).`,
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

			target := args[0]
			examples, err := resolveExamples(ctx, reg, target, version)
			if err != nil {
				return err
			}
			if len(examples) == 0 {
				fmt.Println("No code examples found")
				return nil
			}

			if jsonOut {
				var items []exampleListItemJSON
				for i, ex := range examples {
					items = append(items, exampleListItemJSON{
						Index:     i,
						Title:     ex.title,
						Languages: ex.languages,
					})
				}
				return ui.PrintJSON(items)
			}

			if cmdutil.Interactive() {
				listItems := make([]registryItem, len(examples))
				for i, ex := range examples {
					listItems[i] = registryItem{
						title:      fmt.Sprintf("%-60s  %s", ex.title, strings.Join(ex.languages, ", ")),
						filterText: ex.title,
						value:      i,
					}
				}
				idx, err := runInteractiveList(
					fmt.Sprintf("%d examples", len(examples)),
					fmt.Sprintf("%-60s  %s", "TITLE", "LANGUAGES"),
					listItems)
				if err != nil {
					return err
				}
				if idx >= 0 {
					langs := examples[idx].languages
					lang := langs[0]
					fmt.Printf("\nTip: pulumi registry example get %s %d --language %s\n", target, idx, lang)
				}
				return nil
			}

			// Non-interactive mode: print list with --limit.
			shown := examples
			if limit > 0 && len(shown) > limit {
				shown = shown[:limit]
			}
			for i, ex := range shown {
				fmt.Printf("  %d. %s (%s)\n", i, ex.title, strings.Join(ex.languages, ", "))
			}
			if len(shown) < len(examples) {
				fmt.Printf("\nShowing %d of %d example(s). Use --limit to see more.\n", len(shown), len(examples))
			} else {
				fmt.Printf("\nTotal: %d example(s)\n", len(examples))
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package-or-token"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific package version")
	cmd.PersistentFlags().IntVar(&limit, "limit", 50, "Maximum number of examples to show (0 for all)")

	return cmd
}
