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

//go:build integration
// +build integration

package main

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRandomProviderIntegration tests the MCP server with the random provider.
// This test requires the random provider to be installed.
//
// Run with: go test -tags integration -v
func TestRandomProviderIntegration(t *testing.T) {
	ctx := context.Background()

	// Create a session
	logCallback := func(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
		t.Logf("[%v] %s", sev, msg)
	}
	session, err := NewSession(ctx, logCallback, logCallback)
	require.NoError(t, err)
	defer session.Close()

	// Configure the random provider
	providerId, err := session.AddProvider("random-1", "random", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "random-1", providerId)

	// Get the schema
	schema, err := session.GetSchema(providerId)
	require.NoError(t, err)
	assert.NotNil(t, schema)
	assert.Equal(t, "random", schema.Name)

	// Get resource schema for random:index/randomString:RandomString
	resourceSchema, err := ExtractResourceSchema(schema, "random:index/randomString:RandomString")
	require.NoError(t, err)
	assert.NotNil(t, resourceSchema)

	// Get the provider
	provider, err := session.GetProvider(providerId)
	require.NoError(t, err)

	// Create a random string
	urn := resource.NewURN(
		"test-stack",
		"test-project",
		"",
		"random:index/randomString:RandomString",
		"test-random",
	)

	inputs := resource.PropertyMap{
		"length": resource.NewNumberProperty(16),
	}

	// Check inputs
	checkResp, err := provider.Check(ctx, plugin.CheckRequest{
		URN:  urn,
		News: inputs,
		Olds: resource.PropertyMap{},
	})
	require.NoError(t, err)
	assert.Empty(t, checkResp.Failures)

	// Create the resource
	createResp, err := provider.Create(ctx, plugin.CreateRequest{
		URN:        urn,
		Properties: checkResp.Properties,
		Timeout:    300,
		Preview:    false,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, createResp.ID)
	_, hasResult := createResp.Properties["result"]
	assert.True(t, hasResult)

	result := createResp.Properties["result"]
	assert.True(t, result.IsString())
	assert.Equal(t, 16, len(result.StringValue()))

	t.Logf("Created random string with ID: %s", createResp.ID)
	t.Logf("Random value: %s", result.StringValue())

	// Read the resource back
	readResp, err := provider.Read(ctx, plugin.ReadRequest{
		URN:    urn,
		ID:     createResp.ID,
		Inputs: checkResp.Properties,
		State:  createResp.Properties,
	})
	require.NoError(t, err)
	assert.Equal(t, createResp.ID, readResp.ID)
	assert.Equal(t, result.StringValue(), readResp.Outputs["result"].StringValue())

	// Delete the resource
	_, err = provider.Delete(ctx, plugin.DeleteRequest{
		URN:     urn,
		ID:      createResp.ID,
		Outputs: createResp.Properties,
		Timeout: 300,
	})
	require.NoError(t, err)

	t.Log("Successfully deleted random string")
}

// TestMultipleProviderConfigurations tests that multiple configurations of the same provider work correctly.
func TestMultipleProviderConfigurations(t *testing.T) {
	ctx := context.Background()

	// Create a session
	logCallback := func(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
		t.Logf("[%v] %s", sev, msg)
	}
	session, err := NewSession(ctx, logCallback, logCallback)
	require.NoError(t, err)
	defer session.Close()

	// Configure first random provider
	providerId1, err := session.AddProvider("random-1", "random", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "random-1", providerId1)

	// Configure second random provider
	providerId2, err := session.AddProvider("random-2", "random", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "random-2", providerId2)

	// Both should be accessible
	provider1, err := session.GetProvider(providerId1)
	require.NoError(t, err)
	assert.NotNil(t, provider1)

	provider2, err := session.GetProvider(providerId2)
	require.NoError(t, err)
	assert.NotNil(t, provider2)

	// They should be different instances
	assert.NotEqual(t, provider1, provider2)
}

// TestSchemaCache verifies that schemas are cached correctly.
func TestSchemaCache(t *testing.T) {
	ctx := context.Background()

	// Create a session
	logCallback := func(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
		t.Logf("[%v] %s", sev, msg)
	}
	session, err := NewSession(ctx, logCallback, logCallback)
	require.NoError(t, err)
	defer session.Close()

	// Configure provider
	providerId, err := session.AddProvider("random-1", "random", "", nil)
	require.NoError(t, err)

	// Get schema first time
	schema1, err := session.GetSchema(providerId)
	require.NoError(t, err)
	assert.NotNil(t, schema1)

	// Get schema second time (should be from cache)
	schema2, err := session.GetSchema(providerId)
	require.NoError(t, err)
	assert.NotNil(t, schema2)

	// Should be the exact same instance (from cache)
	assert.Equal(t, schema1, schema2)
}
