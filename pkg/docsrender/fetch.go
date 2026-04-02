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

package docsrender

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

// FetchArgs configures how docs content is fetched.
type FetchArgs struct {
	Client          *http.Client
	DocsBaseURL     string
	RegistryBaseURL string
}

func (a FetchArgs) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a FetchArgs) docsBase() string {
	if a.DocsBaseURL != "" {
		return strings.TrimRight(a.DocsBaseURL, "/")
	}
	return DefaultDocsBaseURL
}

func (a FetchArgs) registryBase() string {
	if a.RegistryBaseURL != "" {
		return strings.TrimRight(a.RegistryBaseURL, "/")
	}
	return DefaultRegistryBaseURL
}

// FetchDocsPage fetches a docs page by path. Returns nil, nil for 404.
func FetchDocsPage(ctx context.Context, args FetchArgs, path string) ([]byte, error) {
	path = strings.Trim(path, "/")
	url := fmt.Sprintf("%s/docs/%s/index.md", args.docsBase(), path)
	return fetchMarkdown(ctx, args.client(), url)
}

// FetchRegistryPage fetches a registry page by path. Returns nil, nil for 404.
func FetchRegistryPage(ctx context.Context, args FetchArgs, path string) ([]byte, error) {
	path = strings.Trim(path, "/")
	// Strip leading "registry/" if present since we add it in the URL.
	path = strings.TrimPrefix(path, "registry/")
	url := fmt.Sprintf("%s/registry/%s/index.md", args.registryBase(), path)
	return fetchMarkdown(ctx, args.client(), url)
}

func fetchMarkdown(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}
	return body, nil
}

func userAgent() string {
	return fmt.Sprintf("pulumi-cli/1 (%s; %s)", version.Version, runtime.GOOS)
}

// StripFrontmatter removes YAML frontmatter delimited by "---\n" from content.
func StripFrontmatter(content []byte) []byte {
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return content
	}

	// Skip past the opening "---\n" or "---\r\n".
	rest := content[3:]
	idx := bytes.Index(rest, []byte("\n"))
	if idx < 0 {
		return content
	}
	rest = rest[idx+1:]

	// Find the closing "---" delimiter.
	closeLF := bytes.Index(rest, []byte("\n---\n"))
	closeCRLF := bytes.Index(rest, []byte("\r\n---\r\n"))

	switch {
	case closeLF >= 0 && (closeCRLF < 0 || closeLF <= closeCRLF):
		return rest[closeLF+5:]
	case closeCRLF >= 0:
		return rest[closeCRLF+7:]
	default:
		return content
	}
}
