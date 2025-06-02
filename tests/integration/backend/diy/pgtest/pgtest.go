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

// Package pgtest contains supporting code for running tests that hit PostgreSQL.
package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/pulumi/pulumi/tests/integration/backend/diy/docker"
)

// Database owns state for running and shutting down PostgreSQL test containers.
type Database struct {
	container    docker.Container
	connString   string
	databaseName string
}

// New creates a new test PostgreSQL database inside a Docker container.
// The container is automatically cleaned up when the test completes.
func New(t *testing.T) *Database {
	t.Helper()

	// Generate unique identifiers for this test
	containerName := "pulumi-pgtest-" + GenerateID() + "-" + strconv.FormatInt(time.Now().Unix(), 10)
	dbName := "pulumitest_" + GenerateID()

	// PostgreSQL Docker settings
	image := "postgres:17"
	port := "5432"
	password := "testpassword"

	dockerArgs := []string{
		"-e", "POSTGRES_PASSWORD=" + password,
		"-e", "POSTGRES_DB=" + dbName,
		"-e", "POSTGRES_HOST_AUTH_METHOD=trust", // Allow connections without password for postgres user
	}

	appArgs := []string{
		"-c", "log_statement=all",
		"-c", "shared_buffers=128MB",
		"-c", "max_connections=100",
	}

	// Start the container
	container, err := docker.StartContainer(image, containerName, port, dockerArgs, appArgs)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	t.Logf("Started PostgreSQL container: %s (ID: %s)", container.Name, container.ID)
	t.Logf("Host port: %s", container.HostPort)

	// Wait for PostgreSQL to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := docker.WaitForReady(ctx, container.HostPort, 30*time.Second); err != nil {
		if stopErr := docker.StopContainer(container.ID); stopErr != nil {
			t.Logf("Warning: failed to stop container after startup failure: %v", stopErr)
		}
		t.Fatalf("PostgreSQL container failed to become ready: %v", err)
	}

	// Build connection string
	host, port, err := net.SplitHostPort(container.HostPort)
	if err != nil {
		if stopErr := docker.StopContainer(container.ID); stopErr != nil {
			t.Logf("Warning: failed to stop container after host port parsing failure: %v", stopErr)
		}
		t.Fatalf("Failed to parse host port: %v", err)
	}

	connString := fmt.Sprintf("postgres://postgres:%s@%s:%s/%s?sslmode=disable",
		password, host, port, dbName)

	// Verify we can connect
	db, err := sql.Open("postgres", connString)
	if err != nil {
		if stopErr := docker.StopContainer(container.ID); stopErr != nil {
			t.Logf("Warning: failed to stop container after connection failure: %v", stopErr)
		}
		t.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Wait for database to be fully ready
	for i := 0; i < 30; i++ {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		if i == 29 {
			logs := docker.DumpContainerLogs(container.ID)
			if stopErr := docker.StopContainer(container.ID); stopErr != nil {
				t.Logf("Warning: failed to stop container after ping failures: %v", stopErr)
			}
			t.Fatalf("Database failed to become ready after 30 attempts. Container logs:\n%s", logs)
		}
		time.Sleep(time.Second)
	}

	t.Logf("PostgreSQL is ready. Connection string: postgres://postgres:****@%s:%s/%s?sslmode=disable",
		host, port, dbName)

	// Set up cleanup
	t.Cleanup(func() {
		t.Helper()
		t.Logf("Stopping PostgreSQL container: %s", container.Name)
		if err := docker.StopContainer(container.ID); err != nil {
			t.Logf("Warning: failed to stop container: %v", err)
		}
	})

	return &Database{
		container:    container,
		connString:   connString,
		databaseName: dbName,
	}
}

// ConnectionString returns the PostgreSQL connection string for this test database.
func (d *Database) ConnectionString() string {
	return d.connString
}

// ConnectionStringWithTable returns a connection string with a specific table parameter.
func (d *Database) ConnectionStringWithTable(tableName string) string {
	return d.connString + "&table=" + tableName
}

// GenerateID creates a short random string suitable for use as a unique identifier.
func GenerateID() string {
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to current time if crypto/rand fails
		return strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
	}
	return hex.EncodeToString(bytes)
}
