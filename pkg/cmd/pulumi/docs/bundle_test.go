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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIDocsPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		wantPkg string
		wantKey string
		wantOK  bool
	}{
		{
			name:    "resource in module",
			path:    "registry/packages/aws/api-docs/s3/bucket",
			wantPkg: "aws",
			wantKey: "s3/bucket",
			wantOK:  true,
		},
		{
			name:    "root-level resource",
			path:    "registry/packages/random/api-docs/randomstring",
			wantPkg: "random",
			wantKey: "randomstring",
			wantOK:  true,
		},
		{
			name:    "nested module resource",
			path:    "registry/packages/aws/api-docs/ec2/transitgateway/route",
			wantPkg: "aws",
			wantKey: "ec2/transitgateway/route",
			wantOK:  true,
		},
		{
			name:    "api-docs index",
			path:    "registry/packages/aws/api-docs",
			wantPkg: "aws",
			wantKey: "",
			wantOK:  true,
		},
		{
			name:    "package root - no api-docs",
			path:    "registry/packages/aws",
			wantPkg: "",
			wantKey: "",
			wantOK:  false,
		},
		{
			name:    "non-registry path",
			path:    "docs/iac/concepts/stacks",
			wantPkg: "",
			wantKey: "",
			wantOK:  false,
		},
		{
			name:    "leading and trailing slashes",
			path:    "/registry/packages/aws/api-docs/s3/bucket/",
			wantPkg: "aws",
			wantKey: "s3/bucket",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pkg, key, ok := ParseAPIDocsPath(tt.path)
			assert.Equal(t, tt.wantOK, ok, "ok")
			assert.Equal(t, tt.wantPkg, pkg, "packageName")
			assert.Equal(t, tt.wantKey, key, "docKey")
		})
	}
}

func TestLookupBundleDoc(t *testing.T) {
	t.Parallel()
	bundle := &CLIDocsBundle{
		Resources: map[string]string{
			"s3/bucket":    "# aws.s3.Bucket\n\nA bucket resource.",
			"randomstring": "# RandomString\r\n\r\nA random string.\tDone.",
		},
		Functions: map[string]string{
			"s3/getBucket": "# aws.s3.getBucket\n\nGet a bucket.",
		},
	}

	t.Run("resource found", func(t *testing.T) {
		t.Parallel()
		body, title, found := LookupBundleDoc(bundle, "s3/bucket")
		assert.True(t, found)
		assert.Equal(t, "aws.s3.Bucket", title)
		assert.Equal(t, "A bucket resource.", body)
	})

	t.Run("function found", func(t *testing.T) {
		t.Parallel()
		body, title, found := LookupBundleDoc(bundle, "s3/getBucket")
		assert.True(t, found)
		assert.Equal(t, "aws.s3.getBucket", title)
		assert.Equal(t, "Get a bucket.", body)
	})

	t.Run("normalizes line endings and tabs", func(t *testing.T) {
		t.Parallel()
		body, title, found := LookupBundleDoc(bundle, "randomstring")
		assert.True(t, found)
		assert.Equal(t, "RandomString", title)
		assert.Contains(t, body, "    Done.") // tab replaced with spaces
		assert.NotContains(t, body, "\r\n")
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		_, _, found := LookupBundleDoc(bundle, "nonexistent")
		assert.False(t, found)
	})
}

func TestBundleNavItems(t *testing.T) {
	t.Parallel()
	bundle := &CLIDocsBundle{
		Package: "aws",
		Resources: map[string]string{
			"s3/bucket":              "# Bucket\n\nContent",
			"s3/bucketPolicy":       "# BucketPolicy\n\nContent",
			"ec2/instance":           "# Instance\n\nContent",
			"ec2/vpc":                "# Vpc\n\nContent",
			"ec2/transitgateway/route": "# Route\n\nContent",
			"rootresource":           "# RootResource\n\nContent",
		},
		Functions: map[string]string{
			"s3/getBucket": "# getBucket\n\nContent",
			"rootfunc":     "# rootFunc\n\nContent",
		},
	}

	t.Run("module level - s3", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "s3", "aws")

		// Should have: Bucket, BucketPolicy (resources), getBucket (function)
		assert.Len(t, items, 3)

		labels := make([]string, len(items))
		for i, item := range items {
			labels[i] = item.label
		}
		assert.Contains(t, labels, "🔗 Bucket")
		assert.Contains(t, labels, "🔗 BucketPolicy")
		assert.Contains(t, labels, "🔗 getBucket")

		// Check paths
		for _, item := range items {
			assert.True(t, len(item.path) > 0)
			assert.Contains(t, item.path, "registry/packages/aws/api-docs/s3/")
		}
	})

	t.Run("module level - ec2 with sub-modules", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "ec2", "aws")

		labels := make([]string, len(items))
		for i, item := range items {
			labels[i] = item.label
		}

		// Should have: transitgateway (sub-module drill), Instance, Vpc (resources)
		assert.Contains(t, labels, "🔗 transitgateway"+navDrill)
		assert.Contains(t, labels, "🔗 Instance")
		assert.Contains(t, labels, "🔗 Vpc")
	})

	t.Run("root level - empty prefix", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "", "aws")

		labels := make([]string, len(items))
		for i, item := range items {
			labels[i] = item.label
		}

		// Should have sub-modules: ec2, s3 (drill), plus root-level: RootResource, rootFunc
		assert.Contains(t, labels, "🔗 ec2"+navDrill)
		assert.Contains(t, labels, "🔗 s3"+navDrill)
		assert.Contains(t, labels, "🔗 RootResource")
		assert.Contains(t, labels, "🔗 rootFunc")
	})

	t.Run("nested module", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "ec2/transitgateway", "aws")

		assert.Len(t, items, 1)
		assert.Equal(t, "🔗 Route", items[0].label)
		assert.Equal(t, "registry/packages/aws/api-docs/ec2/transitgateway/route", items[0].path)
	})

	t.Run("empty bundle", func(t *testing.T) {
		t.Parallel()
		empty := &CLIDocsBundle{}
		items := BundleNavItems(empty, "s3", "aws")
		assert.Empty(t, items)
	})
}

func TestFetchCLIDocsBundle(t *testing.T) {
	testBundle := CLIDocsBundle{
		Version:        1,
		Package:        "testpkg",
		PackageVersion: "1.0.0",
		Resources: map[string]string{
			"myresource": "# MyResource\n\nHello",
		},
		Functions: map[string]string{
			"myfunction": "# myFunction\n\nWorld",
		},
	}

	t.Run("successful fetch", func(t *testing.T) {
		// Clear any cached entry for this test
		bundleMemCache.Delete("testpkg-fetch")

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/registry/packages/testpkg-fetch/api-docs/cli-docs.json", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(testBundle)
		}))
		defer srv.Close()

		t.Setenv("PULUMI_HOME", t.TempDir())

		bundle, err := FetchCLIDocsBundle(srv.URL, "testpkg-fetch")
		require.NoError(t, err)
		assert.Equal(t, "testpkg", bundle.Package)
		assert.Equal(t, "1.0.0", bundle.PackageVersion)
		assert.Contains(t, bundle.Resources, "myresource")
		assert.Contains(t, bundle.Functions, "myfunction")

		// Clean up memory cache
		bundleMemCache.Delete("testpkg-fetch")
	})

	t.Run("404 returns BundleNotAvailableError", func(t *testing.T) {
		bundleMemCache.Delete("notfound-pkg")

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		t.Setenv("PULUMI_HOME", t.TempDir())

		_, err := FetchCLIDocsBundle(srv.URL, "notfound-pkg")
		require.Error(t, err)
		var bundleErr *BundleNotAvailableError
		assert.ErrorAs(t, err, &bundleErr)
		assert.Equal(t, "notfound-pkg", bundleErr.Package)
	})

	t.Run("in-memory cache hit", func(t *testing.T) {
		// Pre-populate memory cache
		cacheKey := "mem-cached-pkg"
		bundleMemCache.Store(cacheKey, &testBundle)
		defer bundleMemCache.Delete(cacheKey)

		// Server should not be called
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("should not have made HTTP request — cache should have been used")
		}))
		defer srv.Close()

		bundle, err := FetchCLIDocsBundle(srv.URL, cacheKey)
		require.NoError(t, err)
		assert.Equal(t, "testpkg", bundle.Package)
	})
}

func TestDiskCacheRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, bundleCacheDir)
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))

	bundle := &CLIDocsBundle{
		Version:        1,
		Package:        "testpkg",
		PackageVersion: "2.0.0",
		Resources:      map[string]string{"res": "# Res\n\nContent"},
		Functions:      map[string]string{"fn": "# Fn\n\nContent"},
	}

	t.Setenv("PULUMI_HOME", tmpDir)

	// Save
	err := saveBundleToDisk("testpkg", bundle)
	require.NoError(t, err)

	// Verify files exist
	assert.FileExists(t, filepath.Join(cacheDir, "testpkg.json"))
	assert.FileExists(t, filepath.Join(cacheDir, "testpkg.meta.json"))

	// Load
	loaded, err := loadBundleFromDisk("testpkg")
	require.NoError(t, err)
	assert.Equal(t, "testpkg", loaded.Package)
	assert.Equal(t, "2.0.0", loaded.PackageVersion)
	assert.Contains(t, loaded.Resources, "res")
	assert.Contains(t, loaded.Functions, "fn")
}

func TestDiskCacheTTLExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, bundleCacheDir)
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))

	t.Setenv("PULUMI_HOME", tmpDir)

	// Write an expired metadata file
	meta := bundleCacheMeta{
		PackageVersion: "1.0.0",
		FetchedAt:      time.Now().Add(-2 * time.Hour), // 2 hours ago, past the 1-hour TTL
	}
	metaData, _ := json.Marshal(meta)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "expired.meta.json"), metaData, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "expired.json"), []byte(`{}`), 0o600))

	_, err := loadBundleFromDisk("expired")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache expired")
}

func TestExtractBundleTitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "MyResource", extractBundleTitle("# MyResource\n\nContent"))
	assert.Equal(t, "aws.s3.Bucket", extractBundleTitle("# aws.s3.Bucket\n\nContent"))
	assert.Equal(t, "", extractBundleTitle("No title here"))
	assert.Equal(t, "SingleLine", extractBundleTitle("# SingleLine"))
}

func TestExtractBundleDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "basic description",
			content: "# Bucket\n\nProvides an S3 bucket resource. This is great.",
			want:    "Provides an S3 bucket resource.",
		},
		{
			name:    "description without period",
			content: "# Bucket\n\nManages an S3 bucket",
			want:    "Manages an S3 bucket",
		},
		{
			name:    "skips deprecation notice",
			content: "# Old\n\n> **Deprecated:** Use New instead.\n\nThe old resource.",
			want:    "The old resource.",
		},
		{
			name:    "no description",
			content: "# Bucket\n\n## Section",
			want:    "",
		},
		{
			name:    "title only",
			content: "# Bucket",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractBundleDescription(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterCodeBlocksByLanguage(t *testing.T) {
	t.Parallel()

	multiLangExample := "## Example Usage\n\n" +
		"```typescript\nconsole.log('hello');\n```\n\n\n\n" +
		"```python\nprint('hello')\n```\n\n\n\n" +
		"```go\nfmt.Println(\"hello\")\n```\n\n" +
		"## Next Section\n"

	t.Run("filters to python", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "python")
		assert.Contains(t, result, "```python")
		assert.Contains(t, result, "print('hello')")
		assert.NotContains(t, result, "```typescript")
		assert.NotContains(t, result, "```go")
		assert.Contains(t, result, "## Next Section")
	})

	t.Run("filters to go", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "go")
		assert.Contains(t, result, "```go")
		assert.NotContains(t, result, "```typescript")
		assert.NotContains(t, result, "```python")
	})

	t.Run("no filter when language is empty", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "")
		assert.Equal(t, multiLangExample, result)
	})

	t.Run("keeps all when language not found", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "rust")
		// All blocks kept since none match
		assert.Contains(t, result, "```typescript")
		assert.Contains(t, result, "```python")
		assert.Contains(t, result, "```go")
	})

	t.Run("preserves isolated code blocks", func(t *testing.T) {
		t.Parallel()
		content := "Some text\n\n```bash\necho hello\n```\n\nMore text\n\n```python\nx = 1\n```\n"
		result := FilterCodeBlocksByLanguage(content, "python")
		// These are separated by non-blank text, so they're independent
		assert.Contains(t, result, "```bash")
		assert.Contains(t, result, "```python")
	})
}

func TestRenderBundleTable(t *testing.T) {
	t.Parallel()

	t.Run("root level with modules", func(t *testing.T) {
		t.Parallel()
		bundle := &CLIDocsBundle{
			Package: "aws",
			Resources: map[string]string{
				"provider":   "# Provider\n\nThe provider type.",
				"s3/bucket":  "# Bucket\n\nAn S3 bucket.",
				"ec2/vpc":    "# Vpc\n\nA VPC.",
				"ec2/subnet": "# Subnet\n\nA subnet.",
			},
			Functions: map[string]string{
				"getarn":      "# getArn\n\nParses an ARN.",
				"s3/getbucket": "# getBucket\n\nLooks up a bucket.",
			},
		}

		table := RenderBundleTable(bundle, "")
		assert.Contains(t, table, "Modules")
		assert.Contains(t, table, "s3")
		assert.Contains(t, table, "ec2")
		assert.Contains(t, table, "Resources")
		assert.Contains(t, table, "Provider")
		assert.Contains(t, table, "Functions")
		assert.Contains(t, table, "getArn")
		// Items in sub-modules should NOT appear at root level
		assert.NotContains(t, table, "Bucket")
		assert.NotContains(t, table, "Vpc")
	})

	t.Run("module level", func(t *testing.T) {
		t.Parallel()
		bundle := &CLIDocsBundle{
			Package: "random",
			Resources: map[string]string{
				"randomstring":   "# RandomString\n\nGenerates a random string.",
				"randompassword": "# RandomPassword\n\nGenerates a random password.",
			},
			Functions: map[string]string{
				"getrandomstring": "# getRandomString\n\nLooks up a random string.",
			},
		}

		table := RenderBundleTable(bundle, "")
		assert.Contains(t, table, "Resources")
		assert.Contains(t, table, "RandomString")
		assert.Contains(t, table, "Generates a random string.")
		assert.Contains(t, table, "Functions")
		assert.Contains(t, table, "getRandomString")
	})
}

func TestClassifyBundleKeys(t *testing.T) {
	t.Parallel()

	bundle := &CLIDocsBundle{
		Package: "aws",
		Resources: map[string]string{
			"s3/bucket":        "# Bucket\n\nContent",
			"s3/bucketPolicy":  "# BucketPolicy\n\nContent",
			"ec2/instance":     "# Instance\n\nContent",
			"rootresource":     "# RootResource\n\nContent",
		},
		Functions: map[string]string{
			"s3/getBucket": "# getBucket\n\nContent",
			"rootfunc":     "# rootFunc\n\nContent",
		},
	}

	t.Run("root level", func(t *testing.T) {
		t.Parallel()
		ck := classifyBundleKeys(bundle, "")
		assert.Contains(t, ck.subModules, "s3")
		assert.Contains(t, ck.subModules, "ec2")
		require.Len(t, ck.resources, 1)
		assert.Equal(t, "RootResource", ck.resources[0].title)
		require.Len(t, ck.functions, 1)
		assert.Equal(t, "rootFunc", ck.functions[0].title)
	})

	t.Run("specific module", func(t *testing.T) {
		t.Parallel()
		ck := classifyBundleKeys(bundle, "s3")
		assert.Empty(t, ck.subModules)
		require.Len(t, ck.resources, 2)
		require.Len(t, ck.functions, 1)
	})

	t.Run("module with no keys", func(t *testing.T) {
		t.Parallel()
		ck := classifyBundleKeys(bundle, "nonexistent")
		assert.Empty(t, ck.subModules)
		assert.Empty(t, ck.resources)
		assert.Empty(t, ck.functions)
	})

	t.Run("empty bundle", func(t *testing.T) {
		t.Parallel()
		empty := &CLIDocsBundle{}
		ck := classifyBundleKeys(empty, "")
		assert.Empty(t, ck.subModules)
		assert.Empty(t, ck.resources)
		assert.Empty(t, ck.functions)
	})
}

func TestReplaceBundleSections(t *testing.T) {
	t.Parallel()

	bundle := &CLIDocsBundle{
		Package: "aws",
		Resources: map[string]string{
			"s3/bucket": "# Bucket\n\nAn S3 bucket.",
		},
		Functions: map[string]string{
			"s3/getBucket": "# getBucket\n\nLooks up a bucket.",
		},
	}

	t.Run("replaces sections", func(t *testing.T) {
		t.Parallel()
		body := "# aws\n\nOverview.\n\n## Modules\n\nOld module list.\n\n" +
			"## Resources\n\nOld resource list.\n\n## Other\n\nUnchanged."
		result := ReplaceBundleSections(body, bundle, "")
		assert.Contains(t, result, "## Modules")
		assert.Contains(t, result, "s3")
		assert.NotContains(t, result, "Old module list.")
		assert.Contains(t, result, "## Other")
		assert.Contains(t, result, "Unchanged.")
	})

	t.Run("no matching sections unchanged", func(t *testing.T) {
		t.Parallel()
		body := "# Page\n\n## Overview\n\nJust text."
		result := ReplaceBundleSections(body, bundle, "")
		assert.Equal(t, body, result)
	})
}

func TestBundleSectionNav(t *testing.T) {
	t.Parallel()

	bundle := &CLIDocsBundle{
		Package: "aws",
		Resources: map[string]string{
			"s3/bucket":    "# Bucket\n\nContent",
			"rootresource": "# RootResource\n\nContent",
		},
		Functions: map[string]string{
			"rootfunc": "# rootFunc\n\nContent",
		},
	}

	t.Run("returns nav for sections", func(t *testing.T) {
		t.Parallel()
		nav := BundleSectionNav(bundle, "", "aws")
		assert.Contains(t, nav, sectionModules)
		assert.Contains(t, nav, sectionResources)
		assert.Contains(t, nav, sectionFunctions)
		require.Len(t, nav[sectionModules], 1)
		assert.Contains(t, nav[sectionModules][0].label, "s3")
	})

	t.Run("empty bundle returns empty map", func(t *testing.T) {
		t.Parallel()
		empty := &CLIDocsBundle{}
		nav := BundleSectionNav(empty, "", "test")
		assert.Empty(t, nav)
	})
}
