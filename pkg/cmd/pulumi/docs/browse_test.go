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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "multi-segment", path: "iac/concepts/stacks", want: "iac/concepts"},
		{name: "single segment", path: "iac", want: ""},
		{name: "registry packages collapses", path: "registry/packages", want: "registry"},
		{name: "registry package child", path: "registry/packages/aws", want: "registry"},
		{name: "trailing slashes", path: "/iac/concepts/", want: "iac"},
		{name: "empty string", path: "", want: ""},
		{name: "deep path", path: "a/b/c/d", want: "a/b/c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, parentPath(tt.path))
		})
	}
}

func TestPathLastSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "multi-segment", path: "iac/concepts/stacks", want: "stacks"},
		{name: "single segment", path: "iac", want: "iac"},
		{name: "trailing slash", path: "iac/concepts/", want: "concepts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, pathLastSegment(tt.path))
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
		{name: "both query and fragment", href: "/docs/install?a=1#top", want: "install"},
		{name: "bare path", href: "/docs/", want: "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, hrefToPath(tt.href))
		})
	}
}

func TestResolveRegistryPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "package only", args: []string{"aws"}, want: "registry/packages/aws"},
		{name: "install", args: []string{"aws", "install"}, want: "registry/packages/aws/installation-configuration"},
		{
			name: "configuration",
			args: []string{"aws", "configuration"},
			want: "registry/packages/aws/installation-configuration",
		},
		{name: "api", args: []string{"aws", "api"}, want: "registry/packages/aws/api-docs"},
		{name: "api with module", args: []string{"aws", "api", "s3"}, want: "registry/packages/aws/api-docs/s3"},
		{name: "unknown subpage", args: []string{"aws", "changelog"}, want: "registry/packages/aws/changelog"},
		{name: "api-docs alias", args: []string{"aws", "api-docs"}, want: "registry/packages/aws/api-docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveRegistryPath(tt.args))
		})
	}
}

func TestFindPage(t *testing.T) {
	t.Parallel()

	pages := []SitemapPage{
		{
			Title: "Concepts",
			Path:  "/docs/concepts/",
			Children: []SitemapPage{
				{Title: "Stacks", Path: "/docs/concepts/stacks/"},
				{Title: "Projects", Path: "/docs/concepts/projects/"},
			},
		},
		{Title: "Install", Path: "/docs/install/"},
	}

	t.Run("found at root", func(t *testing.T) {
		t.Parallel()
		p := findPage(pages, "/docs/install/")
		require.NotNil(t, p)
		assert.Equal(t, "Install", p.Title)
	})

	t.Run("found in children", func(t *testing.T) {
		t.Parallel()
		p := findPage(pages, "/docs/concepts/stacks/")
		require.NotNil(t, p)
		assert.Equal(t, "Stacks", p.Title)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		p := findPage(pages, "/docs/nonexistent/")
		assert.Nil(t, p)
	})
}

func TestFindExactChildren(t *testing.T) {
	t.Parallel()

	pages := []SitemapPage{
		{
			Title: "Concepts",
			Path:  "/docs/concepts/",
			Children: []SitemapPage{
				{Title: "Stacks", Path: "/docs/concepts/stacks/"},
			},
		},
	}

	t.Run("has children", func(t *testing.T) {
		t.Parallel()
		children := findExactChildren(pages, "/docs/concepts/")
		require.Len(t, children, 1)
		assert.Equal(t, "Stacks", children[0].Title)
	})

	t.Run("no children", func(t *testing.T) {
		t.Parallel()
		children := findExactChildren(pages, "/docs/concepts/stacks/")
		assert.Empty(t, children)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		children := findExactChildren(pages, "/docs/missing/")
		assert.Nil(t, children)
	})
}

func TestCollectDescendants(t *testing.T) {
	t.Parallel()

	pages := []SitemapPage{
		{
			Title: "Concepts",
			Path:  "/docs/concepts/",
			Children: []SitemapPage{
				{Title: "Stacks", Path: "/docs/concepts/stacks/"},
				{Title: "Projects", Path: "/docs/concepts/projects/"},
			},
		},
		{Title: "Install", Path: "/docs/install/"},
	}

	t.Run("finds nested descendants", func(t *testing.T) {
		t.Parallel()
		var result []SitemapPage
		collectDescendants(pages, "/docs/concepts/", &result)
		require.Len(t, result, 2)
	})

	t.Run("skips exact match", func(t *testing.T) {
		t.Parallel()
		var result []SitemapPage
		collectDescendants(pages, "/docs/concepts/", &result)
		for _, p := range result {
			assert.NotEqual(t, "/docs/concepts/", p.Path)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		t.Parallel()
		var result []SitemapPage
		collectDescendants(pages, "/docs/missing/", &result)
		assert.Empty(t, result)
	})
}

func TestSitemapToNavOptions(t *testing.T) {
	t.Parallel()

	t.Run("children get drill suffix", func(t *testing.T) {
		t.Parallel()
		pages := []SitemapPage{
			{Title: "Concepts", Path: "/docs/concepts/", Children: []SitemapPage{{Title: "Child"}}},
			{Title: "Install", Path: "/docs/install/"},
		}
		opts := sitemapToNavOptions(pages, "https://www.pulumi.com")
		assert.Equal(t, "Concepts"+navDrill, opts[0].label)
		assert.Equal(t, "Install", opts[1].label)
	})

	t.Run("external URLs preserved", func(t *testing.T) {
		t.Parallel()
		pages := []SitemapPage{
			{Title: "GitHub", Path: "https://github.com/pulumi/pulumi"},
		}
		opts := sitemapToNavOptions(pages, "")
		assert.Equal(t, "https://github.com/pulumi/pulumi", opts[0].href)
	})

	t.Run("html paths get full href", func(t *testing.T) {
		t.Parallel()
		pages := []SitemapPage{
			{Title: "API Reference", Path: "/docs/reference/pkg/nodejs/pulumi/aws/index.html"},
		}
		opts := sitemapToNavOptions(pages, "https://www.pulumi.com")
		assert.Equal(t, "https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/aws/index.html", opts[0].href)
	})
}

func TestChildNavOptions(t *testing.T) {
	t.Parallel()

	pages := []SitemapPage{
		{
			Title: "Concepts",
			Path:  "/docs/concepts/",
			Children: []SitemapPage{
				{Title: "Stacks", Path: "/docs/concepts/stacks/"},
			},
		},
	}

	t.Run("exact match returns children", func(t *testing.T) {
		t.Parallel()
		opts := childNavOptions(pages, "/docs/concepts/", "")
		require.Len(t, opts, 1)
		assert.Equal(t, "Stacks", opts[0].label)
	})

	t.Run("no exact match falls back to descendants", func(t *testing.T) {
		t.Parallel()
		// "/docs/concepts" (no trailing slash) won't match exactly
		opts := childNavOptions(pages, "/docs/concepts", "")
		// collectDescendants should find stacks
		require.Len(t, opts, 1)
	})
}

func TestBuildBrowseMenu(t *testing.T) {
	t.Parallel()

	headings := []heading{
		{level: 2, text: "Overview", slug: "overview"},
		{level: 2, text: "Details", slug: "details"},
	}

	t.Run("root has no Up", func(t *testing.T) {
		t.Parallel()
		menu := buildBrowseMenu(nil, true, false, false, true, -1, nil)
		assert.NotContains(t, menu, navUp)
		assert.Contains(t, menu, navDone)
		assert.NotContains(t, menu, navHome)
	})

	t.Run("non-root has Up and Home", func(t *testing.T) {
		t.Parallel()
		menu := buildBrowseMenu(nil, false, false, false, true, -1, nil)
		assert.Contains(t, menu, navUp)
		assert.Contains(t, menu, navHome)
	})

	t.Run("with sections shows Sections option", func(t *testing.T) {
		t.Parallel()
		menu := buildBrowseMenu(nil, false, true, false, true, -1, headings)
		assert.Contains(t, menu, navSections)
	})

	t.Run("with history shows Back", func(t *testing.T) {
		t.Parallel()
		menu := buildBrowseMenu(nil, false, false, true, true, -1, nil)
		assert.Contains(t, menu, navBack)
	})

	t.Run("section view shows prev/next", func(t *testing.T) {
		t.Parallel()
		menu := buildBrowseMenu(nil, false, true, false, true, 0, headings)
		// At section 0, should have next but not prev (intro is the prev when hasIntro)
		found := false
		for _, item := range menu {
			if len(item) >= len(navNext) && item[:len(navNext)] == navNext {
				found = true
			}
		}
		assert.True(t, found, "should have next section option")
	})

	t.Run("nav items included", func(t *testing.T) {
		t.Parallel()
		items := []navOption{{label: "My Link", path: "some/path"}}
		menu := buildBrowseMenu(items, true, false, false, true, -1, nil)
		assert.Contains(t, menu, "My Link")
	})
}

func TestWebURL(t *testing.T) {
	t.Parallel()

	t.Run("docs path", func(t *testing.T) {
		t.Parallel()
		url := webURL("https://www.pulumi.com", "iac/concepts/stacks")
		assert.Equal(t, "https://www.pulumi.com/docs/iac/concepts/stacks/", url)
	})

	t.Run("registry path", func(t *testing.T) {
		t.Parallel()
		url := webURL("https://www.pulumi.com", "registry/packages/aws")
		assert.Equal(t, "https://www.pulumi.com/registry/packages/aws/", url)
	})
}

func TestNumberedNavLinks(t *testing.T) {
	t.Parallel()

	t.Run("numbered labels", func(t *testing.T) {
		t.Parallel()
		links := []docLink{
			{text: "Stacks", href: "/docs/stacks"},
			{text: "Projects", href: "/docs/projects"},
		}
		opts := numberedNavLinks(links)
		require.Len(t, opts, 2)
		assert.Equal(t, "🔗1 Stacks", opts[0].label)
		assert.Equal(t, "🔗2 Projects", opts[1].label)
		assert.Equal(t, "stacks", opts[0].path)
	})

	t.Run("empty returns nil", func(t *testing.T) {
		t.Parallel()
		opts := numberedNavLinks(nil)
		assert.Nil(t, opts)
	})

	t.Run("backticks stripped from label", func(t *testing.T) {
		t.Parallel()
		links := []docLink{{text: "`pulumi up`", href: "/docs/up"}}
		opts := numberedNavLinks(links)
		assert.Equal(t, "🔗1 pulumi up", opts[0].label)
	})
}
