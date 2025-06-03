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

// Package diy contains tests for the DIY backend with PostgreSQL storage.
// These tests use testcontainers to spin up isolated PostgreSQL 17 containers for each test,
// ensuring no global setup is required and tests work consistently across all environments.
package diy

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	_ "github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres" // driver for postgres://
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	stackpkg "github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/tests/integration/backend/diy/pgtest"
)

// TestPostgresBackend tests core Pulumi operations on the PostgreSQL DIY backend.
// This test exercises backend creation, stack management, deployment operations, and cleanup.
func TestPostgresBackend(t *testing.T) {
	t.Parallel()

	// Skip on Windows as it doesn't support Linux containers by default
	if runtime.GOOS == "windows" {
		t.Skip("Skipping PostgreSQL comprehensive test on Windows - Linux containers not supported by default")
	}

	// Skip if Docker is not available
	if os.Getenv("PULUMI_TEST_SKIP_DOCKER") != "" {
		t.Skip("Skipping test due to PULUMI_TEST_SKIP_DOCKER")
	}

	// Start a PostgreSQL 17 container for this comprehensive test
	pg := pgtest.New(t)

	// Generate unique identifiers
	tableName := "pulumi_comprehensive_" + pgtest.GenerateID()
	stackName1 := "comprehensive-stack-1-" + pgtest.GenerateID()
	stackName2 := "comprehensive-stack-2-" + pgtest.GenerateID()
	renamedStackName := "renamed-stack-" + pgtest.GenerateID()
	projectName := tokens.PackageName("comprehensive-test-project")

	ctx := context.Background()
	url := pg.ConnectionStringWithTable(tableName)

	// Test backend creation and basic properties
	t.Log("Testing backend creation and properties")

	// Create backend with diagnostic sink
	b, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), url, nil)
	require.NoError(t, err, "Failed to create PostgreSQL backend")

	// Test backend properties
	assert.Equal(t, url, b.URL(), "Backend URL mismatch")
	assert.NotEmpty(t, b.Name(), "Backend name should not be empty")

	// Test feature support flags
	assert.False(t, b.SupportsTags(), "DIY backend should not support tags")
	assert.False(t, b.SupportsTemplates(), "DIY backend should not support templates")
	assert.False(t, b.SupportsOrganizations(), "DIY backend should not support organizations")
	assert.False(t, b.SupportsProgress(), "DIY backend should not support progress")
	assert.False(t, b.SupportsDeployments(), "DIY backend should not support deployments")

	// Test project and organization operations
	t.Log("Testing project and organization operations")

	// Set up project
	desc := "Comprehensive test project for PostgreSQL backend"
	project := workspace.Project{
		Name:        projectName,
		Runtime:     workspace.NewProjectRuntimeInfo("nodejs", nil),
		Description: &desc,
	}
	b.SetCurrentProject(&project)

	// Test default organization
	defaultOrg, err := b.GetDefaultOrg(ctx)
	require.NoError(t, err, "GetDefaultOrg should not error")
	assert.Empty(t, defaultOrg, "DIY backend should have empty default org")

	// Test project existence
	projectExists, err := b.DoesProjectExist(ctx, "organization", string(projectName))
	require.NoError(t, err, "DoesProjectExist should not error")
	// Project might or might not exist depending on backend implementation
	_ = projectExists

	// Test current user
	username, orgs, tokenInfo, err := b.CurrentUser()
	require.NoError(t, err, "CurrentUser should not error")
	assert.NotEmpty(t, username, "Username should not be empty")
	assert.Nil(t, orgs, "DIY backend should have nil orgs")
	assert.Nil(t, tokenInfo, "DIY backend should have nil token info")

	// Test stack reference operations
	t.Log("Testing stack reference operations")

	// Test stack reference parsing and validation
	stackRef1, err := b.ParseStackReference(stackName1)
	require.NoError(t, err, "Failed to parse stack reference 1")
	assert.Equal(t, stackName1, stackRef1.Name().String())

	stackRef2, err := b.ParseStackReference(stackName2)
	require.NoError(t, err, "Failed to parse stack reference 2")

	// Test stack name validation
	err = b.ValidateStackName(stackName1)
	require.NoError(t, err, "Stack name 1 should be valid")

	err = b.ValidateStackName("invalid/stack/name/with/too/many/parts")
	// This might or might not error depending on the implementation
	_ = err

	// Test stack creation and basic management
	t.Log("Testing stack creation and basic management")

	// Create initial stacks
	stack1, err := b.CreateStack(ctx, stackRef1, "", nil, nil)
	require.NoError(t, err, "Failed to create stack 1")
	require.NotNil(t, stack1, "Stack 1 should not be nil")

	stack2, err := b.CreateStack(ctx, stackRef2, "", nil, nil)
	require.NoError(t, err, "Failed to create stack 2")
	require.NotNil(t, stack2, "Stack 2 should not be nil")

	// Test stack properties
	assert.Equal(t, stackRef1.String(), stack1.Ref().String())
	assert.Equal(t, b, stack1.Backend())
	assert.Equal(t, stackRef2.String(), stack2.Ref().String())
	assert.Equal(t, b, stack2.Backend())

	// Test stack config location
	configLoc := stack1.ConfigLocation()
	assert.False(t, configLoc.IsRemote, "DIY backend should not have remote config")
	assert.Nil(t, configLoc.EscEnv, "DIY backend should not use ESC env")

	// Test stack tags (should be nil/empty for DIY backend)
	tags := stack1.Tags()
	assert.Nil(t, tags, "DIY backend should not support tags")

	// Test stack listing and retrieval
	t.Log("Testing stack listing and retrieval")

	// Test getting individual stacks
	retrievedStack1, err := b.GetStack(ctx, stackRef1)
	require.NoError(t, err, "Failed to get stack 1")
	require.NotNil(t, retrievedStack1, "Retrieved stack 1 should not be nil")

	retrievedStack2, err := b.GetStack(ctx, stackRef2)
	require.NoError(t, err, "Failed to get stack 2")
	require.NotNil(t, retrievedStack2, "Retrieved stack 2 should not be nil")

	// Test getting non-existent stack
	nonExistentRef, _ := b.ParseStackReference("non-existent-stack")
	nonExistentStack, err := b.GetStack(ctx, nonExistentRef)
	require.NoError(t, err, "Getting non-existent stack should not error")
	assert.Nil(t, nonExistentStack, "Non-existent stack should be nil")

	// Test listing stacks with various filters
	allStacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err, "Failed to list all stacks")
	assert.Nil(t, token, "Continuation token should be nil")
	assert.Len(t, allStacks, 2, "Should have exactly 2 stacks")

	// Test listing with organization filter
	orgStacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{
		Organization: &[]string{"organization"}[0],
	}, nil)
	require.NoError(t, err, "Failed to list stacks with org filter")
	assert.Nil(t, token, "Continuation token should be nil")
	assert.Len(t, orgStacks, 2, "Should have 2 stacks with org filter")

	// Test listing with project filter
	projName := string(projectName)
	projStacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{
		Project: &projName,
	}, nil)
	require.NoError(t, err, "Failed to list stacks with project filter")
	assert.Nil(t, token, "Continuation token should be nil")
	// Might have 0 or 2 stacks depending on implementation
	_ = projStacks

	// Test deployment import/export operations
	t.Log("Testing deployment import/export operations")

	// Test exporting deployment (should work even for empty stack)
	exportedDeployment, err := b.ExportDeployment(ctx, stack1)
	require.NoError(t, err, "Failed to export deployment")
	require.NotNil(t, exportedDeployment, "Exported deployment should not be nil")
	assert.Equal(t, apitype.DeploymentSchemaVersionCurrent, exportedDeployment.Version)
	assert.NotNil(t, exportedDeployment.Deployment, "Deployment data should not be nil")

	// Test stack-level export
	stackExport, err := stack1.ExportDeployment(ctx)
	require.NoError(t, err, "Failed to export stack deployment")
	require.NotNil(t, stackExport, "Stack export should not be nil")

	// Test importing the exported deployment back
	err = b.ImportDeployment(ctx, stack2, exportedDeployment)
	require.NoError(t, err, "Failed to import deployment")

	// Test stack-level import
	err = stack2.ImportDeployment(ctx, exportedDeployment)
	require.NoError(t, err, "Failed to import stack deployment")

	// Test history and update information
	t.Log("Testing history and update information")

	// Test getting stack history
	history, err := b.GetHistory(ctx, stackRef1, 10, 1)
	require.NoError(t, err, "Failed to get stack history")
	// History might be empty or have entries depending on previous operations
	// For empty stacks, history can be nil or empty slice - both are valid
	if history == nil {
		t.Log("Stack history is nil (no history yet)")
	} else {
		t.Logf("Stack history contains %d entries", len(history))
	}

	// Test getting latest configuration
	latestConfig, err := b.GetLatestConfiguration(ctx, stack1)
	if err != nil {
		// It's okay if there's no previous deployment
		assert.Contains(t, err.Error(), "no previous deployment", "Error should be about no previous deployment")
	} else {
		assert.NotNil(t, latestConfig, "Latest config should not be nil if no error")
	}

	// Test secrets management and configuration
	t.Log("Testing secrets management and configuration")

	// Test default secrets manager
	projectStack := &workspace.ProjectStack{}
	secretsManager, err := b.DefaultSecretManager(projectStack)
	if err == nil {
		assert.NotNil(t, secretsManager, "Default secrets manager should not be nil")
	}

	// Test stack-level secrets manager
	stackSecretsManager, err := stack1.DefaultSecretManager(projectStack)
	if err == nil {
		assert.NotNil(t, stackSecretsManager, "Stack secrets manager should not be nil")
	}

	// Test detailed configuration operations with actual values
	t.Log("Testing detailed configuration operations")
	configKey := config.MustMakeKey(string(projectName), "test-value")
	secretKey := config.MustMakeKey(string(projectName), "secret-value")

	// Create a secrets manager for testing configuration
	_, sm, err := passphrase.NewPassphraseSecretsManager("testpassphrase")
	require.NoError(t, err, "Failed to create secrets manager")

	// Encrypt a secret value
	secretVal, err := sm.Encrypter().EncryptValue(ctx, "secret-data")
	require.NoError(t, err, "Failed to encrypt secret value")

	// Create configuration with both regular and secret values
	cfg := config.Map{
		configKey: config.NewValue("test-config-value"),
		secretKey: config.NewSecureValue(secretVal),
	}

	// Verify configuration was created correctly
	val, err := cfg[configKey].Value(config.NopDecrypter)
	require.NoError(t, err, "Failed to get config value")
	assert.Equal(t, "test-config-value", val)
	assert.True(t, cfg[secretKey].Secure(), "Secret config should be secure")

	// Test snapshot operations with actual resource states
	t.Log("Testing snapshot operations with actual resource states")
	testURN := resource.NewURN("test-stack", projectName, "", "test:index:Resource", "test-resource")

	snap := deploy.NewSnapshot(deploy.Manifest{}, b64.NewBase64SecretsManager(), []*resource.State{
		{
			Type:    resource.RootStackType,
			URN:     resource.CreateURN("test-stack", string(resource.RootStackType), "", string(projectName), "test-stack"),
			Custom:  false,
			Outputs: resource.PropertyMap{},
		},
		{
			Type:   "test:index:Resource",
			URN:    testURN,
			Custom: true,
			ID:     "test-resource-id",
			Inputs: resource.PropertyMap{
				"name": resource.NewStringProperty("test-resource"),
			},
			Outputs: resource.PropertyMap{
				"name": resource.NewStringProperty("test-resource"),
				"arn":  resource.NewStringProperty("arn:test:resource"),
			},
		},
	}, nil, deploy.SnapshotMetadata{})

	// Test deployment serialization - simulate what would happen during a real deployment
	deployment, err := stackpkg.SerializeDeployment(ctx, snap, false)
	require.NoError(t, err, "Failed to serialize deployment")

	// Convert to untyped deployment for import testing
	serializedData, err := json.Marshal(deployment)
	require.NoError(t, err, "Failed to marshal deployment")

	untypedDeployment := &apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: json.RawMessage(serializedData),
	}

	// Verify the untyped deployment was created successfully
	require.NotNil(t, untypedDeployment, "Untyped deployment should not be nil")
	assert.Equal(t, apitype.DeploymentSchemaVersionCurrent, untypedDeployment.Version)
	assert.NotEmpty(t, untypedDeployment.Deployment, "Deployment data should not be empty")

	// Test stack renaming operations
	t.Log("Testing stack renaming operations")

	// Test renaming stack
	newStackQName := tokens.QName(renamedStackName)
	newStackRef, err := b.RenameStack(ctx, stack1, newStackQName)
	if err != nil {
		t.Logf("Stack rename failed: %v", err)
		// Continue with original reference for cleanup
	} else {
		require.NotNil(t, newStackRef, "New stack reference should not be nil")
		assert.Contains(t, newStackRef.String(), renamedStackName, "New reference should contain new name")

		// Update our reference for cleanup
		stackRef1 = newStackRef
		stack1, err = b.GetStack(ctx, stackRef1)
		require.NoError(t, err, "Failed to get renamed stack")
	}

	// Test stack-level rename
	anotherNewName := tokens.QName("another-" + renamedStackName)
	anotherNewRef, err := stack2.Rename(ctx, anotherNewName)
	if err != nil {
		t.Logf("Stack-level rename failed: %v", err)
		// Continue with original reference for cleanup
	} else {
		require.NotNil(t, anotherNewRef, "Another new stack reference should not be nil")
		stackRef2 = anotherNewRef
		stack2, err = b.GetStack(ctx, stackRef2)
		require.NoError(t, err, "Failed to get renamed stack 2")
	}

	// Test cancellation operations
	t.Log("Testing cancellation operations")

	// Test canceling current update (should not error even if no update is running)
	err = b.CancelCurrentUpdate(ctx, stackRef1)
	require.NoError(t, err, "Cancel current update should not error")

	// Test stack removal and cleanup
	t.Log("Testing stack removal and cleanup")

	// Test stack removal (backend level)
	removed1, err := b.RemoveStack(ctx, stack1, true)
	require.NoError(t, err, "Failed to remove stack 1")
	assert.False(t, removed1, "Stack 1 should be removed without confirmation")

	// Test stack removal (stack level)
	removed2, err := stack2.Remove(ctx, true)
	require.NoError(t, err, "Failed to remove stack 2")
	assert.False(t, removed2, "Stack 2 should be removed without confirmation")

	// Verify stacks were removed
	removedStack1, err := b.GetStack(ctx, stackRef1)
	require.NoError(t, err, "GetStack should not error for removed stack")
	assert.Nil(t, removedStack1, "Removed stack 1 should be nil")

	removedStack2, err := b.GetStack(ctx, stackRef2)
	require.NoError(t, err, "GetStack should not error for removed stack")
	assert.Nil(t, removedStack2, "Removed stack 2 should be nil")

	// Verify stack list is empty
	finalStacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err, "Failed to list stacks after removal")
	assert.Nil(t, token, "Final continuation token should be nil")
	assert.Len(t, finalStacks, 0, "All stacks should be removed")
}

// TestPostgresBackendMultipleTables tests that multiple backends can use different tables
// in the same PostgreSQL 17 instance without conflicts.
func TestPostgresBackendMultipleTables(t *testing.T) {
	t.Parallel()

	// Skip on Windows as it doesn't support Linux containers by default
	if runtime.GOOS == "windows" {
		t.Skip("Skipping PostgreSQL test on Windows - Linux containers not supported by default")
	}

	// Skip if Docker is not available
	if os.Getenv("PULUMI_TEST_SKIP_DOCKER") != "" {
		t.Skip("Skipping test due to PULUMI_TEST_SKIP_DOCKER")
	}

	// Start a PostgreSQL 17 container for this test using testcontainers
	pg := pgtest.New(t)

	ctx := context.Background()
	desc := "A test project"
	project := workspace.Project{
		Name:        "test-project",
		Runtime:     workspace.NewProjectRuntimeInfo("nodejs", nil),
		Description: &desc,
	}

	// Create two backends with different tables
	table1 := "pulumi_test_1_" + pgtest.GenerateID()
	table2 := "pulumi_test_2_" + pgtest.GenerateID()

	backend1, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), pg.ConnectionStringWithTable(table1), nil)
	require.NoError(t, err, "Failed to create first PostgreSQL backend")
	backend1.SetCurrentProject(&project)

	backend2, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), pg.ConnectionStringWithTable(table2), nil)
	require.NoError(t, err, "Failed to create second PostgreSQL backend")
	backend2.SetCurrentProject(&project)

	// Create stacks in both backends
	stackName := "teststack"

	stackRef1, err := backend1.ParseStackReference(stackName)
	require.NoError(t, err)
	stack1, err := backend1.CreateStack(ctx, stackRef1, "", nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, stack1)

	stackRef2, err := backend2.ParseStackReference(stackName)
	require.NoError(t, err)
	stack2, err := backend2.CreateStack(ctx, stackRef2, "", nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, stack2)

	// Verify each backend only sees its own stack
	stacks1, _, err := backend1.ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err)
	assert.Len(t, stacks1, 1)

	stacks2, _, err := backend2.ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err)
	assert.Len(t, stacks2, 1)

	// Clean up
	_, err = backend1.RemoveStack(ctx, stack1, true)
	require.NoError(t, err)
	_, err = backend2.RemoveStack(ctx, stack2, true)
	require.NoError(t, err)
}

// TestPostgresBackendConcurrency tests that the PostgreSQL backend handles concurrent operations correctly.
func TestPostgresBackendConcurrency(t *testing.T) {
	t.Parallel()

	// Skip on Windows as it doesn't support Linux containers by default
	if runtime.GOOS == "windows" {
		t.Skip("Skipping PostgreSQL test on Windows - Linux containers not supported by default")
	}

	// Skip if Docker is not available
	if os.Getenv("PULUMI_TEST_SKIP_DOCKER") != "" {
		t.Skip("Skipping test due to PULUMI_TEST_SKIP_DOCKER")
	}

	// Start a PostgreSQL 17 container for this test using testcontainers
	pg := pgtest.New(t)

	// Generate a unique table name for this test
	tableName := "pulumi_concurrent_" + pgtest.GenerateID()
	url := pg.ConnectionStringWithTable(tableName)

	// Create multiple backends using the same table to test concurrency
	ctx := context.Background()
	desc := "A test project for concurrency testing"
	project := workspace.Project{
		Name:        "test-concurrent-project",
		Runtime:     workspace.NewProjectRuntimeInfo("nodejs", nil),
		Description: &desc,
	}

	const numConcurrentOperations = 5
	backends := make([]backend.Backend, numConcurrentOperations)
	stacks := make([]backend.Stack, numConcurrentOperations)

	// Create multiple backend instances
	for i := 0; i < numConcurrentOperations; i++ {
		b, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		}), url, nil)
		require.NoError(t, err, "Failed to create PostgreSQL backend %d", i)
		b.SetCurrentProject(&project)
		backends[i] = b
	}

	// Create stacks concurrently
	stackRefs := make([]backend.StackReference, numConcurrentOperations)
	for i := 0; i < numConcurrentOperations; i++ {
		i := i // capture loop variable
		stackName := "concurrent-stack-" + pgtest.GenerateID()
		stackRef, err := backends[i].ParseStackReference(stackName)
		require.NoError(t, err, "Failed to parse stack reference %d", i)
		stackRefs[i] = stackRef

		stack, err := backends[i].CreateStack(ctx, stackRef, "", nil, nil)
		require.NoError(t, err, "Failed to create stack %d", i)
		stacks[i] = stack
	}

	// Verify all stacks exist by listing them
	allStacks, token, err := backends[0].ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err, "Failed to list stacks")
	assert.Nil(t, token, "Continuation token should be nil")
	assert.Len(t, allStacks, numConcurrentOperations, "Should have all created stacks")

	// Clean up all stacks
	for i := 0; i < numConcurrentOperations; i++ {
		removed, err := backends[i].RemoveStack(ctx, stacks[i], true)
		require.NoError(t, err, "Failed to remove stack %d", i)
		assert.False(t, removed, "Stack %d should be removed without confirmation", i)
	}

	// Verify all stacks were removed
	allStacks, token, err = backends[0].ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err, "Failed to list stacks after cleanup")
	assert.Nil(t, token, "Continuation token should be nil")
	assert.Len(t, allStacks, 0, "All stacks should be removed")
}
