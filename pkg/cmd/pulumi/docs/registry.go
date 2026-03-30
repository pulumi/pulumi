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

package docs

import (
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/spf13/cobra"
)

// newRegistryCmd creates the `pulumi docs registry` subcommand for browsing
// registry package documentation with convenient shorthands.
func (dc *docsCmd) newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry <package> [install|api [module]]",
		Short: "View registry package documentation",
		Long: "Browse Pulumi registry package documentation in the terminal.\n\n" +
			"Shorthands:\n" +
			"  pulumi docs registry <package>              Package overview\n" +
			"  pulumi docs registry <package> install       Installation & configuration\n" +
			"  pulumi docs registry <package> api            API docs overview\n" +
			"  pulumi docs registry <package> api <module>   API docs for a module\n\n" +
			"Full path syntax also works:\n" +
			"  pulumi docs registry/packages/<package>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			// If the first arg looks like a full path (contains /), treat it as such
			if strings.Contains(args[0], "/") {
				path := "registry/" + strings.TrimPrefix(args[0], "registry/")
				return dc.fetchAndRender(path)
			}

			path := resolveRegistryPath(args)
			return dc.fetchAndRender(path)
		},
	}
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package"},
			{Name: "subpage"},
			{Name: "module"},
		},
	})
	return cmd
}

// resolveRegistryPath converts registry subcommand arguments into a content path.
//
//	["aws"]                → "registry/packages/aws"
//	["aws", "install"]     → "registry/packages/aws/installation-configuration"
//	["aws", "api"]         → "registry/packages/aws/api-docs"
//	["aws", "api", "s3"]   → "registry/packages/aws/api-docs/s3"
func resolveRegistryPath(args []string) string {
	pkg := args[0]
	base := "registry/packages/" + pkg

	if len(args) < 2 {
		return base
	}

	switch strings.ToLower(args[1]) {
	case "install", "installation", "config", "configuration":
		return base + "/installation-configuration"
	case "api", "api-docs":
		if len(args) >= 3 {
			return base + "/api-docs/" + args[2]
		}
		return base + "/api-docs"
	default:
		// Treat as a sub-path
		return base + "/" + strings.Join(args[1:], "/")
	}
}
