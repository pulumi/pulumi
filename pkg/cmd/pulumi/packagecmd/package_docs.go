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

func newPackageDocsCmd() *cobra.Command {
	var lang string
	var os string

	cmd := &cobra.Command{
		Use:   "docs <package>[@<version>] <token>",
		Short: "Display documentation for a registry package resource or function",
		Long: `Display documentation for a specific resource or function in a registry package.

The package argument accepts the same formats as 'pulumi package add':
  aws, pulumi/pulumi/aws, aws@7.20.0

The token argument is a Pulumi type token, e.g.:
  random:index/randomPassword:RandomPassword
  aws:s3/bucket:Bucket

Use 'pulumi package nav' to discover available tokens.

If --lang is not specified, the language is inferred from the current
Pulumi project's runtime (Pulumi.yaml).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			pkg, err := parseAndResolvePackage(ctx, args[0])
			if err != nil {
				return err
			}

			token := args[1]

			reg := registryForContext(ctx)
			md, err := reg.GetPackageDocsMarkdown(
				ctx, pkg.Source, pkg.Publisher, pkg.Name, pkg.Version, token,
				docsOpts(lang, os, ""),
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
			{Name: "token", Usage: "<token>"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&lang, "lang", "", "Language for code examples and type names (typescript, python, go, csharp, yaml, java)")
	cmd.Flags().StringVar(&os, "os", "", "OS for platform-specific content (linux, macos, windows)")

	return cmd
}
