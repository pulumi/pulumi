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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

type howtoGuideJSON struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func newRegistryPackageHowtoCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "howto <package>",
		Short: "List how-to guides for a package",
		Long: `List how-to guides available for a package from the Pulumi Registry website.

These guides provide step-by-step tutorials and practical examples.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			packageName := args[0]

			guides, err := fetchHowtoGuides(ctx, packageName)
			if err != nil {
				return err
			}

			if len(guides) == 0 {
				fmt.Printf("No how-to guides found for %q\n", packageName)
				return nil
			}

			if jsonOut {
				return ui.PrintJSON(guides)
			}

			for _, g := range guides {
				fmt.Printf("  - %s\n    %s\n", g.Title, g.URL)
			}
			fmt.Printf("\nTotal: %d guide(s)\n", len(guides))
			return nil
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

// githubContentEntry represents a file entry from the GitHub Contents API.
type githubContentEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func fetchHowtoGuides(ctx context.Context, packageName string) ([]howtoGuideJSON, error) {
	// Fetch the guide file listing from the pulumi/registry GitHub repo.
	url := fmt.Sprintf(
		"https://api.github.com/repos/pulumi/registry/contents/themes/default/content/registry/packages/%s/how-to-guides",
		packageName,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "pulumi-cli")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching how-to guides: %w", err)
	}
	defer contract.IgnoreClose(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching how-to guides: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var entries []githubContentEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	var guides []howtoGuideJSON
	for _, entry := range entries {
		if entry.Type != "file" || entry.Name == "_index.md" || !strings.HasSuffix(entry.Name, ".md") {
			continue
		}
		slug := strings.TrimSuffix(entry.Name, ".md")
		title := slugToTitle(slug)
		guides = append(guides, howtoGuideJSON{
			Slug:  slug,
			Title: title,
			URL:   fmt.Sprintf("https://www.pulumi.com/registry/packages/%s/how-to-guides/%s/", packageName, slug),
		})
	}
	return guides, nil
}

// slugToTitle converts a filename slug like "aws-ts-hello-fargate" to a readable title.
func slugToTitle(slug string) string {
	// Replace hyphens with spaces and title-case.
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
