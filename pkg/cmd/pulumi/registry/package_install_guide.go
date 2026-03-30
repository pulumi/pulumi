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
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

// installGuideJSON is the JSON output format for the install-guide command.
type installGuideJSON struct {
	Package string `json:"package"`
	Content string `json:"content"`
}

func newRegistryPackageInstallGuideCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "install-guide <package>",
		Short: "Show installation and configuration guide for a package",
		Long: `Show the installation and configuration guide for a package from the Pulumi Registry.

This fetches the guide content directly from the Pulumi Registry GitHub repository
and displays it in the terminal.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			packageName := args[0]

			url := fmt.Sprintf(
				"https://raw.githubusercontent.com/pulumi/registry/master/themes/default/content/registry/packages/%s/installation-configuration.md",
				packageName,
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("creating request: %w", err)
			}
			req.Header.Set("User-Agent", "pulumi-cli")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("fetching installation guide: %w", err)
			}
			defer contract.IgnoreClose(resp.Body)

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("No installation guide found for %q\n", packageName)
				return nil
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("fetching installation guide: HTTP %d", resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}

			content := stripFrontmatter(string(body))
			content = stripShortcodes(content)

			if jsonOut {
				return ui.PrintJSON(installGuideJSON{
					Package: packageName,
					Content: content,
				})
			}

			return ui.RenderMarkdown(content)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

// stripFrontmatter removes YAML frontmatter delimited by --- from the content.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	// Find the closing --- after the opening one.
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return content
	}
	// Skip past the closing --- and any immediately following newline.
	result := content[3+end+3:]
	result = strings.TrimLeft(result, "\r\n")
	return result
}

// shortcodePattern matches Hugo shortcodes in both {{< >}} and {{% %}} forms.
var shortcodePattern = regexp.MustCompile(`\{\{[<%]\s*.*?\s*[>%]\}\}`)

// stripShortcodes removes Hugo shortcodes from the content.
func stripShortcodes(content string) string {
	return shortcodePattern.ReplaceAllString(content, "")
}
