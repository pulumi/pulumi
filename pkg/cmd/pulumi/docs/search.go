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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// cliConfig is the structure served by the docs site at /docs/cli-config.json.
type cliConfig struct {
	Algolia algoliaConfig `json:"algolia"`
}

type algoliaConfig struct {
	AppID     string `json:"appId"`
	SearchKey string `json:"searchKey"`
	IndexName string `json:"indexName"`
}

type algoliaResponse struct {
	Hits []algoliaHit `json:"hits"`
}

type algoliaHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	H1          string `json:"h1"`
	Description string `json:"description"`
	Href        string `json:"href"`
	Section     string `json:"section"`
}

func (h algoliaHit) displayTitle() string {
	title := h.H1
	if title == "" {
		title = h.Title
	}
	if title == "" {
		title = h.Href
	}
	return title
}

// sectionName returns "registry" or "docs" based on the hit's href.
func (h algoliaHit) sectionName() string {
	if strings.Contains(h.Href, "/registry/") {
		return "registry"
	}
	return "docs"
}

// sectionTag returns a short tag like "[docs]" or "[registry]" for display in search results.
func (h algoliaHit) sectionTag() string {
	return "[" + h.sectionName() + "]"
}

// fetchCLIConfig fetches the CLI config file from the docs site.
func fetchCLIConfig(baseURL string) (*cliConfig, error) {
	configURL := strings.TrimRight(baseURL, "/") + "/docs/cli-config.json"

	//nolint:gosec // URL is constructed from user-provided base URL
	resp, err := http.Get(configURL)
	if err != nil {
		return nil, fmt.Errorf("fetching CLI config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CLI config not available (status %d) — "+
			"set ALGOLIA_APP_ID and ALGOLIA_APP_SEARCH_KEY as a fallback", resp.StatusCode)
	}

	var cfg cliConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decoding CLI config: %w", err)
	}
	return &cfg, nil
}

// getAlgoliaCredentials resolves Algolia credentials. It checks env vars first,
// then fetches cli-config.json from the docs site.
func getAlgoliaCredentials(baseURL string) (appID, apiKey, indexName string, err error) {
	// Env vars take precedence (useful for overrides and testing)
	appID = os.Getenv("ALGOLIA_APP_ID")
	apiKey = os.Getenv("ALGOLIA_APP_SEARCH_KEY")
	if appID != "" && apiKey != "" {
		indexName = os.Getenv("ALGOLIA_INDEX_NAME")
		if indexName == "" {
			indexName = "production"
		}
		return appID, apiKey, indexName, nil
	}

	// Fetch from docs site
	cfg, err := fetchCLIConfig(baseURL)
	if err != nil {
		return "", "", "", err
	}

	if cfg.Algolia.AppID == "" || cfg.Algolia.SearchKey == "" {
		return "", "", "", errors.New("CLI config missing Algolia credentials")
	}

	indexName = cfg.Algolia.IndexName
	if indexName == "" {
		indexName = "production"
	}
	return cfg.Algolia.AppID, cfg.Algolia.SearchKey, indexName, nil
}

func searchDocs(query, appID, apiKey, indexName string) ([]algoliaHit, error) {
	searchURL := fmt.Sprintf("https://%s-dsn.algolia.net/1/indexes/%s/query", appID, indexName)

	params := url.Values{}
	params.Set("query", query)
	params.Set("hitsPerPage", "10")
	params.Set("facetFilters", `[["section:Docs","section:Registry"]]`)

	reqBody, err := json.Marshal(map[string]string{
		"params": params.Encode(),
	})
	if err != nil {
		return nil, fmt.Errorf("encoding search request: %w", err)
	}

	req, err := http.NewRequest("POST", searchURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}
	req.Header.Set("X-Algolia-Application-Id", appID)
	req.Header.Set("X-Algolia-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search returned status %d: %s", resp.StatusCode, string(body))
	}

	var result algoliaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return result.Hits, nil
}

func runSearch(cmd *docsCmd, query string) error {
	appID, apiKey, indexName, err := getAlgoliaCredentials(cmd.baseURL)
	if err != nil {
		return err
	}

	hits, err := searchDocs(query, appID, apiKey, indexName)
	if err != nil {
		return err
	}

	if len(hits) == 0 {
		if cmd.jsonOutput {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No results found.")
		return nil
	}

	if cmd.jsonOutput {
		type searchResult struct {
			Title       string `json:"title"`
			Section     string `json:"section"`
			Description string `json:"description,omitempty"`
			Href        string `json:"href"`
		}
		results := make([]searchResult, len(hits))
		for i, hit := range hits {
			results[i] = searchResult{
				Title:       hit.displayTitle(),
				Section:     hit.sectionName(),
				Description: hit.Description,
				Href:        hit.Href,
			}
		}
		return ui.PrintJSON(results)
	}

	interactive := cmdutil.Interactive()

	if interactive {
		// Build selectable list with title on first line and description indented on second.
		options := make([]string, len(hits))
		for i, hit := range hits {
			tag := hit.sectionTag()
			desc := hit.Description
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			if desc != "" {
				options[i] = fmt.Sprintf("%s %s\n     %s", tag, hit.displayTitle(), desc)
			} else {
				options[i] = fmt.Sprintf("%s %s", tag, hit.displayTitle())
			}
		}

		selected := ui.PromptUser(
			"Select a page to view:",
			options,
			options[0],
			cmdutil.GetGlobalColorization(),
		)

		for i, opt := range options {
			if opt == selected {
				return cmd.viewPage(hits[i].Href)
			}
		}
		return nil
	}

	// Non-interactive: print list
	for i, hit := range hits {
		fmt.Printf("%d. %s %s\n", i+1, hit.sectionTag(), hit.displayTitle())
		if hit.Description != "" {
			fmt.Printf("   %s\n", hit.Description)
		}
		fmt.Printf("   %s\n\n", hit.Href)
	}
	return nil
}
