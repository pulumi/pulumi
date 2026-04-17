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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/spf13/cobra"
)

func newPackageNavCmd() *cobra.Command {
	var lang string
	var search string

	cmd := &cobra.Command{
		Use:   "nav <package>[@<version>]",
		Short: "Browse a registry package's modules, resources, and functions",
		Long: `Browse a registry package's modules, resources, and functions.

The package argument accepts the same formats as 'pulumi package add':
  aws, pulumi/pulumi/aws, aws@7.20.0

If --lang is not specified, the language is inferred from the current
Pulumi project's runtime (Pulumi.yaml).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			pkg, err := parseAndResolvePackage(ctx, args[0])
			if err != nil {
				return err
			}

			reg := registryForContext(ctx)
			md, err := reg.GetPackageNavMarkdown(
				ctx, pkg.Source, pkg.Publisher, pkg.Name, pkg.Version,
				docsOpts(lang, "", search),
			)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), md)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package", Usage: "<package>[@<version>]"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&lang, "lang", "", "Language for display names (typescript, python, go, csharp, yaml, java)")
	cmd.Flags().StringVar(&search, "search", "", "Filter items by name (case-insensitive substring)")

	return cmd
}
