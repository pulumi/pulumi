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
// These tests use Docker to spin up isolated PostgreSQL containers for each test,
// ensuring no global setup is required and tests work consistently across all environments.
package diy

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	_ "github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres" // Import to register PostgreSQL provider
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/tests/integration/backend/diy/pgtest"
)

// TestPostgresBackend tests basic functionality of the PostgreSQL DIY backend.
// This test automatically starts a PostgreSQL Docker container for isolated testing.
func TestPostgresBackend(t *testing.T) {
	t.Parallel()

	// Skip if Docker is not available
	if os.Getenv("PULUMI_TEST_SKIP_DOCKER") != "" {
		t.Skip("Skipping test due to PULUMI_TEST_SKIP_DOCKER")
	}

	// Start a PostgreSQL container for this test
	pg := pgtest.New(t)

	// Generate a unique table name for this test
	tableName := "pulumi_test_" + pgtest.GenerateID()
	url := pg.ConnectionStringWithTable(tableName)

	// Create a new PostgreSQL backend
	ctx := context.Background()
	b, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), url, nil)
	require.NoError(t, err, "Failed to create PostgreSQL backend")

	// Verify the backend was created successfully
	assert.Equal(t, url, b.URL(), "Backend URL does not match")

	// Create a new stack
	stackName := "teststack" + pgtest.GenerateID()
	desc := "A test project"
	project := workspace.Project{
		Name:        "test-project",
		Runtime:     workspace.NewProjectRuntimeInfo("nodejs", nil),
		Description: &desc,
	}
	b.SetCurrentProject(&project)

	// Parse stack reference
	stackRef, err := b.ParseStackReference(stackName)
	require.NoError(t, err, "Failed to parse stack reference")

	// Create the stack
	stack, err := b.CreateStack(ctx, stackRef, "", nil, nil)
	require.NoError(t, err, "Failed to create stack")
	assert.NotNil(t, stack, "Stack should not be nil")

	// Get the stack
	getStack, err := b.GetStack(ctx, stackRef)
	require.NoError(t, err, "Failed to get stack")
	assert.NotNil(t, getStack, "Stack should not be nil")

	// List stacks
	stacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil)
	require.NoError(t, err, "Failed to list stacks")
	assert.Nil(t, token, "Continuation token should be nil")
	assert.Len(t, stacks, 1, "There should be exactly one stack")

	// Remove the stack
	removed, err := b.RemoveStack(ctx, stack, true)
	require.NoError(t, err, "Failed to remove stack")
	assert.False(t, removed, "Stack should be removed without confirmation")

	// Verify the stack was removed
	getStack, err = b.GetStack(ctx, stackRef)
	require.NoError(t, err, "GetStack should not return error for nonexistent stack")
	assert.Nil(t, getStack, "Stack should be nil after removal")
}

// TestPostgresBackendMultipleTables tests that multiple backends can use different tables
// in the same PostgreSQL instance without conflicts.
func TestPostgresBackendMultipleTables(t *testing.T) {
	t.Parallel()

	// Skip if Docker is not available
	if os.Getenv("PULUMI_TEST_SKIP_DOCKER") != "" {
		t.Skip("Skipping test due to PULUMI_TEST_SKIP_DOCKER")
	}

	// Start a PostgreSQL container for this test
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
