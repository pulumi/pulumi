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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		raw       string
		wantBody  string
		wantTitle string
	}{
		{
			name:      "basic frontmatter with title",
			raw:       "---\ntitle: Getting Started\n---\nHello world.",
			wantBody:  "Hello world.",
			wantTitle: "Getting Started",
		},
		{
			name:      "quoted title",
			raw:       "---\ntitle: \"Hello World\"\n---\nBody here.",
			wantBody:  "Body here.",
			wantTitle: "Hello World",
		},
		{
			name:      "single-quoted title",
			raw:       "---\ntitle: 'My Page'\n---\nBody.",
			wantBody:  "Body.",
			wantTitle: "My Page",
		},
		{
			name:      "no frontmatter",
			raw:       "Just plain markdown.",
			wantBody:  "Just plain markdown.",
			wantTitle: "",
		},
		{
			name:      "unclosed frontmatter",
			raw:       "---\ntitle: Broken\nNo closing delimiter.",
			wantBody:  "---\ntitle: Broken\nNo closing delimiter.",
			wantTitle: "",
		},
		{
			name:      "frontmatter with no title",
			raw:       "---\ndescription: No title here\n---\nBody.",
			wantBody:  "Body.",
			wantTitle: "",
		},
		{
			name:      "leading newlines after frontmatter trimmed",
			raw:       "---\ntitle: Test\n---\n\n\nBody.",
			wantBody:  "Body.",
			wantTitle: "Test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body, title := StripFrontmatter(tt.raw)
			assert.Equal(t, tt.wantBody, body)
			assert.Equal(t, tt.wantTitle, title)
		})
	}
}

func TestExtractContentPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		rawURL string
		base   string
		want   string
	}{
		{
			name:   "docs prefix",
			rawURL: "/docs/iac/concepts/stacks/",
			base:   "https://www.pulumi.com",
			want:   "iac/concepts/stacks",
		},
		{
			name:   "registry prefix",
			rawURL: "/registry/packages/aws/",
			base:   "https://www.pulumi.com",
			want:   "registry/packages/aws",
		},
		{
			name:   "full URL with base",
			rawURL: "https://www.pulumi.com/docs/iac/concepts/",
			base:   "https://www.pulumi.com",
			want:   "iac/concepts",
		},
		{
			name:   "bare path",
			rawURL: "/docs/install/",
			base:   "",
			want:   "install",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractContentPath(tt.rawURL, tt.base)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRegistryPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "registry package", path: "registry/packages/aws", want: true},
		{name: "docs path", path: "docs/iac/concepts", want: false},
		{name: "bare registry", path: "registry", want: true},
		{name: "leading slash", path: "/registry/packages/aws", want: true},
		{name: "trailing slash", path: "registry/", want: true},
		{name: "empty string", path: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isRegistryPath(tt.path))
		})
	}
}

func TestContentPrefix(t *testing.T) {
	t.Parallel()

	t.Run("registry path", func(t *testing.T) {
		t.Parallel()
		prefix, trimmed := contentPrefix("registry/packages/aws")
		assert.Equal(t, "/registry/", prefix)
		assert.Equal(t, "packages/aws", trimmed)
	})

	t.Run("docs path", func(t *testing.T) {
		t.Parallel()
		prefix, trimmed := contentPrefix("iac/concepts/stacks")
		assert.Equal(t, "/docs/", prefix)
		assert.Equal(t, "iac/concepts/stacks", trimmed)
	})

	t.Run("bare registry", func(t *testing.T) {
		t.Parallel()
		prefix, trimmed := contentPrefix("registry")
		assert.Equal(t, "/registry/", prefix)
		assert.Equal(t, "", trimmed)
	})
}

//nolint:paralleltest // HTTP tests use shared package-level httpClient and sitemapCache
func TestFetchDoc(t *testing.T) {
	t.Run("200 OK with frontmatter", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("---\ntitle: My Page\n---\nHello world."))
		}))
		defer srv.Close()

		body, title, err := FetchDoc(srv.URL, "iac/concepts")
		require.NoError(t, err)
		assert.Equal(t, "My Page", title)
		assert.Equal(t, "Hello world.", body)
	})

	t.Run("404 for registry returns RegistryNotAvailableError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		_, _, err := FetchDoc(srv.URL, "registry/packages/aws")
		require.Error(t, err)
		var regErr *RegistryNotAvailableError
		assert.ErrorAs(t, err, &regErr)
	})

	t.Run("403 treated as not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, _, err := FetchDoc(srv.URL, "registry/packages/missing")
		require.Error(t, err)
		var regErr *RegistryNotAvailableError
		assert.ErrorAs(t, err, &regErr)
	})

	t.Run("500 returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, _, err := FetchDoc(srv.URL, "iac/concepts")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 500")
	})
}

//nolint:paralleltest // HTTP tests use shared package-level state
func TestResolveRedirect(t *testing.T) {
	t.Run("HTTP 301 redirect", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "/docs/iac/new-path/")
			w.WriteHeader(http.StatusMovedPermanently)
		}))
		defer srv.Close()

		path, err := resolveRedirect(srv.URL, "iac/old-path")
		require.NoError(t, err)
		assert.Equal(t, "iac/new-path", path)
	})

	t.Run("meta refresh tag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(
				`<html><head><meta http-equiv="refresh" ` +
					`content="0;url=/docs/iac/redirected/"></head></html>`))
		}))
		defer srv.Close()

		path, err := resolveRedirect(srv.URL, "iac/old-page")
		require.NoError(t, err)
		assert.Equal(t, "iac/redirected", path)
	})

	t.Run("404 returns empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		path, err := resolveRedirect(srv.URL, "iac/missing")
		require.NoError(t, err)
		assert.Equal(t, "", path)
	})
}

//nolint:paralleltest // HTTP tests use shared sitemapCache
func TestFetchSitemapJSON(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		pages := []SitemapPage{
			{Title: "Concepts", Path: "/docs/concepts/"},
			{Title: "Install", Path: "/docs/install/"},
		}
		resp := sitemapResponse{Pages: pages}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		sitemapCache.Delete(srv.URL + "/test")

		result, err := fetchSitemapJSON(srv.URL+"/test", "test sitemap")
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Concepts", result[0].Title)

		sitemapCache.Delete(srv.URL + "/test")
	})

	t.Run("404 returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		sitemapCache.Delete(srv.URL + "/missing")
		_, err := fetchSitemapJSON(srv.URL+"/missing", "missing sitemap")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})

	t.Run("caching", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			_ = json.NewEncoder(w).Encode(sitemapResponse{
				Pages: []SitemapPage{{Title: "Cached", Path: "/docs/cached/"}},
			})
		}))
		defer srv.Close()

		url := srv.URL + "/cache-test"
		sitemapCache.Delete(url)

		_, err := fetchSitemapJSON(url, "cache test")
		require.NoError(t, err)

		_, err = fetchSitemapJSON(url, "cache test")
		require.NoError(t, err)

		assert.Equal(t, 1, callCount, "second call should use cache")

		sitemapCache.Delete(url)
	})
}

func TestSitemapPageViewLabel(t *testing.T) {
	t.Parallel()

	t.Run("default label", func(t *testing.T) {
		t.Parallel()
		p := SitemapPage{Title: "Test"}
		assert.Equal(t, "Introduction", p.ViewLabel())
	})

	t.Run("custom label", func(t *testing.T) {
		t.Parallel()
		p := SitemapPage{Title: "Test", SelfLabel: "Overview"}
		assert.Equal(t, "Overview", p.ViewLabel())
	})
}
