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

func TestFetchDoc(t *testing.T) { //nolint:paralleltest // mutates package-level HTTPClient
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/iac/concepts/stacks/index.md":
			fmt.Fprint(w, "---\ntitle: Stacks\n---\n# Stacks\n\nContent here.")
		case "/registry/packages/aws/index.md":
			fmt.Fprint(w, "---\ntitle: AWS Provider\n---\n# AWS\n\nAWS content.")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	origClient := HTTPClient
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = origClient })

	t.Run("docs page found", func(t *testing.T) { //nolint:paralleltest // shares HTTPClient
		body, title, err := FetchDoc(srv.URL, "iac/concepts/stacks")
		require.NoError(t, err)
		assert.Equal(t, "Stacks", title)
		assert.Contains(t, body, "Content here.")
	})

	t.Run("registry page found", func(t *testing.T) { //nolint:paralleltest // shares HTTPClient
		body, title, err := FetchDoc(srv.URL, "registry/packages/aws")
		require.NoError(t, err)
		assert.Equal(t, "AWS Provider", title)
		assert.Contains(t, body, "AWS content.")
	})

	t.Run("docs page not found", func(t *testing.T) { //nolint:paralleltest // shares HTTPClient
		_, _, err := FetchDoc(srv.URL, "nonexistent/path")
		assert.Error(t, err)
	})

	//nolint:paralleltest // shares HTTPClient
	t.Run("registry not found returns RegistryNotAvailableError", func(t *testing.T) {
		_, _, err := FetchDoc(srv.URL, "registry/packages/nonexistent")
		assert.Error(t, err)
		var regErr *RegistryNotAvailableError
		assert.ErrorAs(t, err, &regErr)
	})
}

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantBody  string
		wantTitle string
	}{
		{
			name:      "with frontmatter and title",
			input:     "---\ntitle: Getting Started\n---\nHello world.",
			wantBody:  "Hello world.",
			wantTitle: "Getting Started",
		},
		{
			name:      "quoted title",
			input:     "---\ntitle: \"Hello World\"\n---\nBody here.",
			wantBody:  "Body here.",
			wantTitle: "Hello World",
		},
		{
			name:      "no frontmatter",
			input:     "Just plain markdown.",
			wantBody:  "Just plain markdown.",
			wantTitle: "",
		},
		{
			name:      "unclosed frontmatter",
			input:     "---\ntitle: Hello\n",
			wantBody:  "---\ntitle: Hello\n",
			wantTitle: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body, title := StripFrontmatter(tt.input)
			assert.Equal(t, tt.wantBody, body)
			assert.Equal(t, tt.wantTitle, title)
		})
	}
}

func TestHrefToPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		href string
		want string
	}{
		{name: "docs prefix stripped", href: "/docs/iac/concepts/stacks/", want: "iac/concepts/stacks"},
		{name: "registry prefix kept", href: "/registry/packages/aws/", want: "registry/packages/aws"},
		{name: "query params stripped", href: "/docs/install?ref=nav", want: "install"},
		{name: "fragment stripped", href: "/docs/install#linux", want: "install"},
		{name: "bare path", href: "/docs/", want: "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, HrefToPath(tt.href))
		})
	}
}
