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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchDocsPage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/iac/concepts/stacks/index.md":
			fmt.Fprint(w, "# Stacks\n\nContent here.")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	args := FetchArgs{
		Client:      srv.Client(),
		DocsBaseURL: srv.URL,
	}

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		body, err := FetchDocsPage(t.Context(), args, "iac/concepts/stacks")
		require.NoError(t, err)
		assert.Equal(t, "# Stacks\n\nContent here.", string(body))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		body, err := FetchDocsPage(t.Context(), args, "nonexistent/path")
		require.NoError(t, err)
		assert.Nil(t, body)
	})
}

func TestFetchRegistryPage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/packages/aws/index.md":
			fmt.Fprint(w, "# AWS Provider")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	args := FetchArgs{
		Client:          srv.Client(),
		RegistryBaseURL: srv.URL,
	}

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		body, err := FetchRegistryPage(t.Context(), args, "registry/packages/aws")
		require.NoError(t, err)
		assert.Equal(t, "# AWS Provider", string(body))
	})

	t.Run("path without registry prefix", func(t *testing.T) {
		t.Parallel()
		body, err := FetchRegistryPage(t.Context(), args, "packages/aws")
		require.NoError(t, err)
		assert.Equal(t, "# AWS Provider", string(body))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		body, err := FetchRegistryPage(t.Context(), args, "packages/nonexistent")
		require.NoError(t, err)
		assert.Nil(t, body)
	})
}

func TestFetchHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	args := FetchArgs{
		Client:      srv.Client(),
		DocsBaseURL: srv.URL,
	}

	_, err := FetchDocsPage(t.Context(), args, "some/page")
	assert.ErrorContains(t, err, "HTTP 500")
}

func TestFetchUserAgent(t *testing.T) {
	t.Parallel()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	args := FetchArgs{
		Client:      srv.Client(),
		DocsBaseURL: srv.URL,
	}
	_, err := FetchDocsPage(t.Context(), args, "test")
	require.NoError(t, err)
	assert.Contains(t, gotUA, "pulumi-cli/1")
}

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with frontmatter",
			input:    "---\ntitle: Hello\nweight: 1\n---\n# Content\n\nBody here.",
			expected: "# Content\n\nBody here.",
		},
		{
			name:     "without frontmatter",
			input:    "# Just Content\n\nNo frontmatter.",
			expected: "# Just Content\n\nNo frontmatter.",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "only frontmatter no close",
			input:    "---\ntitle: Hello\n",
			expected: "---\ntitle: Hello\n",
		},
		{
			name: "frontmatter with windows line endings",
			input: "---\r\ntitle: Hello\r\n" +
				"---\r\n# Content",
			expected: "# Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, string(StripFrontmatter([]byte(tt.input))))
		})
	}
}
