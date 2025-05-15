// Copyright 2025, Pulumi Corporation.
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

package unauthenticatedregistry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	env_core "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPackageSpecifiedVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{
  "name": "random",
  "publisher": "pulumi",
  "source": "pulumi",
  "version": "4.18.0",
  "description": "A Pulumi package to safely use randomness in Pulumi programs.",
  "repoUrl": "https://github.com/pulumi/pulumi-random",
  "category": "cloud",
  "isFeatured": false,
  "packageTypes": [
    "bridged"
  ],
  "packageStatus": "ga",
  "readmeURL": "https://artifacts.pulumi.com/providers/4f280fdc-47eb-43d4-9fd2-bf54eccbbc48/docs/index.md",
  "schemaURL": "https://artifacts.pulumi.com/providers/4f280fdc-47eb-43d4-9fd2-bf54eccbbc48/schema.json",
  "createdAt": "2025-04-17T16:02:41.759Z",
  "visibility": "public"
}
`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := New(diagtest.LogSink(t), env.NewEnv(env_core.MapStore{
		"PULUMI_API": server.URL,
	}))

	pkg, err := client.GetPackage(ctx, "pulumi", "pulumi", "random", &semver.Version{Major: 4, Minor: 18})
	require.NoError(t, err)

	assert.Equal(t, apitype.PackageMetadata{
		Name:          "random",
		Publisher:     "pulumi",
		Source:        "pulumi",
		Version:       semver.Version{Major: 4, Minor: 18},
		Description:   "A Pulumi package to safely use randomness in Pulumi programs.",
		LogoURL:       "",
		RepoURL:       "https://github.com/pulumi/pulumi-random",
		Category:      "cloud",
		IsFeatured:    false,
		PackageTypes:  []apitype.PackageType{"bridged"},
		PackageStatus: apitype.PackageStatusGA,
		ReadmeURL:     "https://artifacts.pulumi.com/providers/4f280fdc-47eb-43d4-9fd2-bf54eccbbc48/docs/index.md",
		SchemaURL:     "https://artifacts.pulumi.com/providers/4f280fdc-47eb-43d4-9fd2-bf54eccbbc48/schema.json",
		CreatedAt:     time.Date(2025, time.April, 17, 16, 2, 41, 759000000, time.UTC),
		Visibility:    apitype.VisibilityPublic,
	}, pkg)
}

func TestGetPackageUnspecifiedVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{
  "name": "random",
  "publisher": "pulumi",
  "source": "pulumi",
  "version": "4.18.2",
  "description": "A Pulumi package to safely use randomness in Pulumi programs.",
  "repoUrl": "https://github.com/pulumi/pulumi-random",
  "category": "cloud",
  "isFeatured": false,
  "packageTypes": [
    "bridged"
  ],
  "packageStatus": "ga",
  "readmeURL": "https://artifacts.pulumi.com/providers/a7592522-ec15-49cf-89f2-3365191ea23b/docs/index.md",
  "schemaURL": "https://artifacts.pulumi.com/providers/a7592522-ec15-49cf-89f2-3365191ea23b/schema.json",
  "createdAt": "2025-04-30T22:44:41.542Z",
  "visibility": "public"
}
`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := New(diagtest.LogSink(t), env.NewEnv(env_core.MapStore{
		"PULUMI_API": server.URL,
	}))

	pkg, err := client.GetPackage(ctx, "pulumi", "pulumi", "random", nil /* latest version */)
	require.NoError(t, err)

	assert.Equal(t, apitype.PackageMetadata{
		Name:      "random",
		Publisher: "pulumi",
		Source:    "pulumi",
		Version: semver.Version{
			Major: 4,
			Minor: 18,
			Patch: 2,
		},
		Description:   "A Pulumi package to safely use randomness in Pulumi programs.",
		RepoURL:       "https://github.com/pulumi/pulumi-random",
		Category:      "cloud",
		PackageTypes:  []apitype.PackageType{"bridged"},
		PackageStatus: apitype.PackageStatusGA,
		ReadmeURL:     "https://artifacts.pulumi.com/providers/a7592522-ec15-49cf-89f2-3365191ea23b/docs/index.md",
		SchemaURL:     "https://artifacts.pulumi.com/providers/a7592522-ec15-49cf-89f2-3365191ea23b/schema.json",
		CreatedAt:     time.Date(2025, time.April, 30, 22, 44, 41, 542000000, time.UTC),
		Visibility:    apitype.VisibilityPublic,
	}, pkg)
}

func TestGetPackageNonExistantPackage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, err := w.Write([]byte(`{
  "code": 404,
  "message": "Not Found: package version 'pulumi/pulumi/does-not-exist@<nil>' not found"
}
`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := New(diagtest.LogSink(t), env.NewEnv(env_core.MapStore{
		"PULUMI_API": server.URL,
	}))

	_, err := client.GetPackage(ctx, "pulumi", "pulumi", "does-not-exist", nil /* latest version */)
	assert.ErrorIs(t, err, backenderr.ErrNotFound)
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

// We never have auth, so we always should return a [backenderr.ErrNotFound] when we get a 404.
func TestGetPackagePrivatePackage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, err := w.Write([]byte(`{
  "code": 404,
  "message": "Not Found: package version 'pulumi/private/missing-credentials@<nil>' not found"
}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := New(diagtest.LogSink(t), env.NewEnv(env_core.MapStore{
		"PULUMI_API": server.URL,
	}))

	_, err := client.GetPackage(ctx, "pulumi", "private", "missing-credentials", nil /* latest version */)
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestSearchByName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{
  "packages": [
    {
      "name": "castai",
      "publisher": "castai",
      "source": "pulumi",
      "version": "0.1.78",
      "title": "CAST AI",
      "description": "A Pulumi package for creating and managing CAST AI cloud resources.",
      "logoUrl": "https://raw.githubusercontent.com/castai/pulumi-castai/main/docs/images/castai-logo.png",
      "repoUrl": "https://github.com/castai/pulumi-castai",
      "category": "cloud",
      "isFeatured": false,
      "packageTypes": [
        "bridged"
      ],
      "packageStatus": "public_preview",
      "readmeURL": "https://artifacts.pulumi.com/providers/8b761e09-343e-4660-ba13-bd24c257fe0e/docs/index.md",
      "schemaURL": "https://artifacts.pulumi.com/providers/8b761e09-343e-4660-ba13-bd24c257fe0e/schema.json",
      "pluginDownloadURL": "github://api.github.com/castai",
      "createdAt": "2025-05-22T17:41:08.019Z",
      "visibility": "public"
    },
    {
      "name": "castai",
      "publisher": "castai",
      "source": "opentofu",
      "version": "7.52.0",
      "description": "A Pulumi provider dynamically bridged from castai.",
      "repoUrl": "https://github.com/castai/terraform-provider-castai",
      "category": "cloud",
      "isFeatured": false,
      "packageTypes": [
        "bridged"
      ],
      "packageStatus": "ga",
      "readmeURL": "https://artifacts.pulumi.com/providers/c56b8d91-3733-4303-b026-7f4d7c66dcc2/docs/index.md",
      "schemaURL": "https://artifacts.pulumi.com/providers/c56b8d91-3733-4303-b026-7f4d7c66dcc2/schema.json",
      "createdAt": "2025-05-10T00:17:43.669Z",
      "visibility": "public"
    }
  ]
}
`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := New(diagtest.LogSink(t), env.NewEnv(env_core.MapStore{
		"PULUMI_API": server.URL,
	}))

	results := []apitype.PackageMetadata{}
	for pkg, err := range client.SearchByName(ctx, ref("castai")) {
		require.NoError(t, err)
		results = append(results, pkg)
	}

	assert.Equal(t, []apitype.PackageMetadata{
		{
			Name:              "castai",
			Publisher:         "castai",
			Source:            "pulumi",
			Version:           semver.Version{Major: 0, Minor: 1, Patch: 78},
			Title:             "CAST AI",
			Description:       "A Pulumi package for creating and managing CAST AI cloud resources.",
			LogoURL:           "https://raw.githubusercontent.com/castai/pulumi-castai/main/docs/images/castai-logo.png",
			RepoURL:           "https://github.com/castai/pulumi-castai",
			Category:          "cloud",
			PackageTypes:      []apitype.PackageType{"bridged"},
			PackageStatus:     apitype.PackageStatusPublicPreview,
			ReadmeURL:         "https://artifacts.pulumi.com/providers/8b761e09-343e-4660-ba13-bd24c257fe0e/docs/index.md",
			SchemaURL:         "https://artifacts.pulumi.com/providers/8b761e09-343e-4660-ba13-bd24c257fe0e/schema.json",
			PluginDownloadURL: "github://api.github.com/castai",
			CreatedAt:         time.Date(2025, time.May, 22, 17, 41, 8, 19000000, time.UTC),
			Visibility:        apitype.VisibilityPublic,
		},
		{
			Name:          "castai",
			Publisher:     "castai",
			Source:        "opentofu",
			Version:       semver.Version{Major: 7, Minor: 52, Patch: 0},
			Description:   "A Pulumi provider dynamically bridged from castai.",
			RepoURL:       "https://github.com/castai/terraform-provider-castai",
			Category:      "cloud",
			PackageTypes:  []apitype.PackageType{"bridged"},
			PackageStatus: apitype.PackageStatusGA,
			ReadmeURL:     "https://artifacts.pulumi.com/providers/c56b8d91-3733-4303-b026-7f4d7c66dcc2/docs/index.md",
			SchemaURL:     "https://artifacts.pulumi.com/providers/c56b8d91-3733-4303-b026-7f4d7c66dcc2/schema.json",
			CreatedAt:     time.Date(2025, time.May, 10, 0, 17, 43, 669000000, time.UTC),
			Visibility:    apitype.VisibilityPublic,
		},
	}, results)
}

func TestSearchByNameNoMatches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"packages":[]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := New(diagtest.LogSink(t), env.NewEnv(env_core.MapStore{
		"PULUMI_API": server.URL,
	}))

	results := []apitype.PackageMetadata{}
	for pkg, err := range client.SearchByName(ctx, ref("404-not-found")) {
		require.NoError(t, err)
		results = append(results, pkg)
	}

	assert.Equal(t, []apitype.PackageMetadata{}, results)
}

func ref[T any](v T) *T { return &v }
