// Copyright 2016-2023, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Make sure that TestingT never diverges from testing.T.
var _ TestingT = (*testing.T)(nil)

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
			assert.Empty(t, test.providers, "test name %s must not use providers", name)
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
		for _, provider := range test.providers {
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
		for _, provider := range test.providers {
			pkg := string(provider.Pkg())
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
