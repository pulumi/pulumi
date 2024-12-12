// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/tests"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Make sure that TestingT never diverges from testing.T.
var _ tests.TestingT = (*testing.T)(nil)

// Ensure that every language test starts with a standard prefix.
func TestTestNames(t *testing.T) {
	t.Parallel()

	for name := range languageTests {
		isInternal := strings.HasPrefix(name, "internal-")
		isl1 := strings.HasPrefix(name, "l1-")
		isl2 := strings.HasPrefix(name, "l2-")
		isl3 := strings.HasPrefix(name, "l3-")
		assert.True(t, isInternal || isl1 || isl2 || isl3, "test name %s must start with internal-, l1-, l2-, or l3-", name)
	}
}

// Ensure l1 tests don't use providers.
func TestL1NoProviders(t *testing.T) {
	t.Parallel()

	for name, test := range languageTests {
		if strings.HasPrefix(name, "l1-") {
			assert.Empty(t, test.Providers, "test name %s must not use providers", name)
		}
	}
}

// Ensure GetTests doesn't return internal- tests.
func TestNoInternalTests(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	engine := &languageTestServer{}

	response, err := engine.GetLanguageTests(ctx, &testingrpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	for _, name := range response.Tests {
		if strings.HasPrefix(name, "internal-") {
			assert.Fail(t, "test name %s must not be returned by GetLanguageTests", name)
		}
	}
}

// Ensure all providers have unique versions, we use this to make dependency checking a lot simpler.
func TestUniqueProviderVersions(t *testing.T) {
	t.Parallel()

	versions := map[string]string{}

	for _, test := range languageTests {
		for _, provider := range test.Providers {
			pkg := string(provider.Pkg())
			version, err := getProviderVersion(provider)
			require.NoError(t, err)

			vstr := version.String()

			if v, ok := versions[vstr]; ok {
				assert.Equal(t, pkg, v, "provider version %s is used by both %s and %s", vstr, pkg, v)
			}
			versions[vstr] = pkg
		}
	}
}

// Ensure all providers report the same version for schema and plugin info
func TestProviderVersions(t *testing.T) {
	t.Parallel()

	for _, test := range languageTests {
		for _, provider := range test.Providers {
			pkg := string(provider.Pkg())
			if pkg == "parameterized" {
				// for parameterized provider, the version is set in the parameterization
				// it is not necessarily the case that the plugin info version is the same as package version
				continue
			}
			version, err := getProviderVersion(provider)
			require.NoError(t, err)

			schema, err := provider.GetSchema(context.Background(), plugin.GetSchemaRequest{})
			require.NoError(t, err)

			var schemaJSON struct {
				Version string `json:"version"`
			}
			err = json.Unmarshal(schema.Schema, &schemaJSON)
			require.NoError(t, err)

			assert.Equal(t, version.String(), schemaJSON.Version,
				"provider %s reports different versions in schema %s and plugin info %s", pkg, version, schemaJSON.Version)
		}
	}
}

// Ensure all providers have valid schemas.
func TestProviderSchemas(t *testing.T) {
	t.Parallel()

	for name, test := range languageTests {
		// Internal tests are allowed to have invalid schemas.
		if strings.HasPrefix(name, "internal-") {
			continue
		}

		loader := &providerLoader{providers: test.Providers}

		for _, provider := range test.Providers {
			if provider.Pkg() == "parameterized" {
				// We don't currently support testing the schemas of parameterized providers.
				continue
			}

			resp, err := provider.GetSchema(context.Background(), plugin.GetSchemaRequest{})
			require.NoError(t, err)

			var pkg schema.PackageSpec
			err = json.Unmarshal(resp.Schema, &pkg)
			require.NoError(t, err)

			_, diags, err := schema.BindSpec(pkg, loader)
			for _, diag := range diags {
				t.Logf("%s: %v", pkg.Name, diag)
			}
			require.NoError(t, err, "bind schema for provider %s: %v", pkg.Name, err)
			require.False(t, diags.HasErrors(), "bind schema for provider %s: %v", pkg.Name, diags)
		}
	}
}

// Ensure all tests have valid programs that can be bound.
func TestBindPrograms(t *testing.T) {
	t.Parallel()

	for name, test := range languageTests {
		// Internal tests are allowed to have invalid programs.
		if strings.HasPrefix(name, "internal-") {
			continue
		}

		src := filepath.Join("testdata", name)
		loader := &providerLoader{providers: test.Providers}
		_, diags, err := pcl.BindDirectory(src, loader)
		for _, diag := range diags {
			t.Logf("%s: %v", name, diag)
		}
		require.NoError(t, err, "bind program for test %s: %v", name, err)
		require.False(t, diags.HasErrors(), "bind program for test %s: %v", name, diags)
	}
}
