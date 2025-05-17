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

// To run this test, first setup a PostgreSQL database:
//
// 1. Install PostgreSQL:
//    ```
//    apt-get update
//    apt-get install -y postgresql postgresql-contrib
//    ```
//
// 2. Start PostgreSQL service:
//    ```
//    service postgresql start
//    ```
//
// 3. Create user and database:
//    ```
//    sudo -u postgres psql -c "CREATE USER pulumi WITH PASSWORD 'pulumi';"
//    sudo -u postgres psql -c "CREATE DATABASE pulumi OWNER pulumi;"
//    sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE pulumi TO pulumi;"
//    ```
//
// 4. Run the test with:
//    ```
//    export PULUMI_TEST_POSTGRES_URL="postgres://pulumi:pulumi@localhost:5432/pulumi?sslmode=disable"
//    go test -v ./tests/integration/backend/diy/...
//    ```
//
// Note: The test will attempt to set up PostgreSQL automatically if PULUMI_TEST_POSTGRES_URL
// is not provided and if it has sufficient permissions.

package diy

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// generateID creates a short random string suitable for use as a unique identifier
func generateID() string {
	// Initialize random source with current time
	rand.Seed(time.Now().UnixNano())

	// Generate a six-character random string
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 6)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// setupPostgreSQL attempts to install and configure PostgreSQL for testing
// Returns a connection string if successful, or an empty string and error if not
func setupPostgreSQL(t *testing.T) (string, error) {
	// Check if PostgreSQL is already installed
	pgVersion := exec.Command("psql", "--version")
	if pgVersion.Run() != nil {
		// PostgreSQL not found, attempt to install it
		t.Log("PostgreSQL not found, attempting to install...")

		// Install PostgreSQL
		installCmd := exec.Command("sh", "-c", "apt-get update && apt-get install -y postgresql postgresql-contrib")
		output, err := installCmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to install PostgreSQL: %v\n%s", err, output)
		}

		// Start PostgreSQL service
		startCmd := exec.Command("service", "postgresql", "start")
		output, err = startCmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to start PostgreSQL service: %v\n%s", err, output)
		}

		t.Log("PostgreSQL installed and started")
	} else {
		t.Log("PostgreSQL is already installed")
	}

	// Create test user and database
	username := "pulumi_test"
	password := "pulumi_test"
	dbname := "pulumi_test_" + generateID()

	// Create user if it doesn't exist
	createUserCmd := exec.Command("sh", "-c", fmt.Sprintf(
		"sudo -u postgres psql -c \"SELECT 1 FROM pg_roles WHERE rolname = '%s'\" | grep -q 1 || "+
			"sudo -u postgres psql -c \"CREATE USER %s WITH PASSWORD '%s';\"",
		username, username, password))
	output, err := createUserCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create PostgreSQL user: %v\n%s", err, output)
	}

	// Create database
	createDBCmd := exec.Command("sh", "-c", fmt.Sprintf(
		"sudo -u postgres psql -c \"CREATE DATABASE %s OWNER %s;\"",
		dbname, username))
	output, err = createDBCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create PostgreSQL database: %v\n%s", err, output)
	}

	// Grant privileges
	grantCmd := exec.Command("sh", "-c", fmt.Sprintf(
		"sudo -u postgres psql -c \"GRANT ALL PRIVILEGES ON DATABASE %s TO %s;\"",
		dbname, username))
	output, err = grantCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to grant privileges: %v\n%s", err, output)
	}

	t.Logf("Created PostgreSQL database '%s' with user '%s'", dbname, username)

	// Return the connection string
	return fmt.Sprintf("postgres://%s:%s@localhost:5432/%s?sslmode=disable",
		username, password, dbname), nil
}

// cleanupPostgres attempts to remove the test database
func cleanupPostgres(t *testing.T, connString string) {
	// Extract database name from connection string
	parts := strings.Split(connString, "/")
	if len(parts) < 4 {
		return
	}
	dbname := strings.Split(parts[3], "?")[0]

	// Drop the database
	dropCmd := exec.Command("sh", "-c", fmt.Sprintf(
		"sudo -u postgres psql -c \"DROP DATABASE IF EXISTS %s;\"", dbname))
	if output, err := dropCmd.CombinedOutput(); err != nil {
		t.Logf("warning: failed to drop test database: %v\n%s", err, output)
	} else {
		t.Logf("Cleaned up test database '%s'", dbname)
	}
}

// TestPostgresBackend tests basic functionality of the PostgreSQL DIY backend.
// This test requires a PostgreSQL instance to be available.
// Set PULUMI_TEST_POSTGRES_URL environment variable to the connection string.
// Example: postgres://pulumi:pulumi@localhost:5432/pulumi?sslmode=disable
func TestPostgresBackend(t *testing.T) {
	postgresURL := os.Getenv("PULUMI_TEST_POSTGRES_URL")
	if postgresURL == "" {
		// Try to set up PostgreSQL automatically
		var err error
		postgresURL, err = setupPostgreSQL(t)
		if err != nil {
			t.Skipf("Skipping PostgreSQL backend test - automatic setup failed: %v", err)
		}
		defer cleanupPostgres(t, postgresURL)
	}

	// Generate a unique table name for this test to avoid conflicts
	tableName := "pulumi_test_" + generateID()
	url := postgresURL + "&table=" + tableName

	// Create a new PostgreSQL backend
	ctx := context.Background()
	backend, err := postgres.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{}), url, nil)
	require.NoError(t, err, "Failed to create PostgreSQL backend")

	// Verify the backend was created successfully
	assert.Equal(t, url, backend.URL(), "Backend URL does not match")

	// Create a new stack
	stackName := "teststack" + generateID()
	project := workspace.Project{
		Name:        "test-project",
		Runtime:     workspace.NewProjectRuntimeInfo("nodejs", nil),
		Description: "A test project",
	}
	backend.SetCurrentProject(&project)

	// Parse stack reference
	stackRef, err := backend.ParseStackReference(stackName)
	require.NoError(t, err, "Failed to parse stack reference")

	// Create the stack
	stack, err := backend.CreateStack(ctx, stackRef, "", nil, nil)
	require.NoError(t, err, "Failed to create stack")
	assert.NotNil(t, stack, "Stack should not be nil")

	// Get the stack
	getStack, err := backend.GetStack(ctx, stackRef)
	require.NoError(t, err, "Failed to get stack")
	assert.NotNil(t, getStack, "Stack should not be nil")

	// List stacks
	stacks, token, err := backend.ListStacks(ctx, nil, nil)
	require.NoError(t, err, "Failed to list stacks")
	assert.Nil(t, token, "Continuation token should be nil")
	assert.Len(t, stacks, 1, "There should be exactly one stack")

	// Remove the stack
	removed, err := backend.RemoveStack(ctx, stack, true)
	require.NoError(t, err, "Failed to remove stack")
	assert.False(t, removed, "Stack should be removed without confirmation")

	// Verify the stack was removed
	getStack, err = backend.GetStack(ctx, stackRef)
	require.NoError(t, err, "GetStack should not return error for nonexistent stack")
	assert.Nil(t, getStack, "Stack should be nil after removal")
}
