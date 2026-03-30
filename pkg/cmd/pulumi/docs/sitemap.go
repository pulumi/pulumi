// Copyright 2024, Pulumi Corporation.
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
	"errors"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/spf13/cobra"
)

func (dc *docsCmd) newSitemapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sitemap [scope]",
		Short: "List available documentation pages",
		Long: "Display the documentation site map as a navigable list of pages.\n\n" +
			"  pulumi docs sitemap                                List all docs pages\n" +
			"  pulumi docs sitemap registry                       List all registry packages\n" +
			"  pulumi docs sitemap registry/packages/<pkg>        List pages for a specific package\n" +
			"  pulumi docs sitemap --json                         Output as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := ""
			if len(args) > 0 {
				scope = strings.Trim(args[0], "/")
			}
			return dc.runSitemap(scope)
		},
	}
	cmd.Flags().BoolVar(&dc.jsonOutput, "json", false, "Output as JSON")
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "scope"}},
	})
	return cmd
}

func (dc *docsCmd) runSitemap(scope string) error {
	pages, err := dc.fetchSitemapForScope(scope)
	if err != nil {
		return err
	}

	if dc.jsonOutput {
		return ui.PrintJSON(pages)
	}

	printSitemapTree(pages, 0)
	return nil
}

func (dc *docsCmd) fetchSitemapForScope(scope string) ([]SitemapPage, error) {
	// Per-package sitemap: registry/packages/<pkg>
	if strings.HasPrefix(scope, "registry/packages/") {
		parts := strings.SplitN(strings.TrimPrefix(scope, "registry/packages/"), "/", 2)
		pkg := parts[0]
		if pkg == "" {
			return nil, errors.New("missing package name — use: pulumi docs sitemap registry/packages/<pkg>")
		}
		return FetchPackageSitemap(dc.registryBaseURL, pkg)
	}

	// Registry top-level
	if scope == "registry" || strings.HasPrefix(scope, "registry/") {
		return FetchRegistrySitemap(dc.registryBaseURL)
	}

	// Default: docs sitemap
	return FetchSitemap(dc.baseURL)
}
