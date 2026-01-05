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
//
// This package uses testcontainers-go to provide isolated PostgreSQL 17 containers
// for each test, ensuring consistent and reliable test environments.
//
// Example usage:
//
//	func TestMyFunction(t *testing.T) {
//		// Start a PostgreSQL 17 container for this test
//		pg := pgtest.New(t)
//
//		// Get connection string for the database
//		connStr := pg.ConnectionString()
//
//		// Or get connection string with a specific table parameter
//		connStrWithTable := pg.ConnectionStringWithTable("my_table")
//
//		// Use the connection string with your database logic
//		db, err := sql.Open("pgx", connStr)
//		require.NoError(t, err)
//		defer db.Close()
//
//		// Container is automatically cleaned up when test completes
//	}
package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL pgx driver
	_ "github.com/lib/pq"              // PostgreSQL pq driver (for compatibility)
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Database owns state for running and shutting down PostgreSQL test containers.
type Database struct {
	container    *postgres.PostgresContainer
	connString   string
	databaseName string
}

// New creates a new test PostgreSQL database using testcontainers.
// The container is automatically cleaned up when the test completes.
func New(t *testing.T) *Database {
	t.Helper()

	// Generate unique identifier for this test
	dbName := "pulumitest_" + GenerateID()
	user := "postgres"
	password := "testpassword"

	ctx := context.Background()

	// Start PostgreSQL 17 container using testcontainers
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(user),
		postgres.WithPassword(password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
		postgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		if termErr := testcontainers.TerminateContainer(postgresContainer); termErr != nil {
			t.Logf("Warning: failed to terminate container after connection string error: %v", termErr)
		}
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Verify we can connect
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		if termErr := testcontainers.TerminateContainer(postgresContainer); termErr != nil {
			t.Logf("Warning: failed to terminate container after connection failure: %v", termErr)
		}
		t.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Wait for database to be fully ready
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for i := 0; i < 30; i++ {
		if err := db.PingContext(pingCtx); err == nil {
			break
		}
		if i == 29 {
			if termErr := testcontainers.TerminateContainer(postgresContainer); termErr != nil {
				t.Logf("Warning: failed to terminate container after ping failures: %v", termErr)
			}
			t.Fatalf("Database failed to become ready after 30 attempts: %v", err)
		}
		time.Sleep(time.Second)
	}

	// Get connection details for logging
	host, err := postgresContainer.Host(ctx)
	if err != nil {
		host = "unknown"
	}
	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		port = "unknown"
	}

	t.Logf("PostgreSQL 17 is ready. Connection: postgres://%s:****@%s:%s/%s?sslmode=disable",
		user, host, port.Port(), dbName)

	// Set up cleanup
	t.Cleanup(func() {
		t.Helper()
		t.Logf("Terminating PostgreSQL container")
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("Warning: failed to terminate container: %v", err)
		}
	})

	return &Database{
		container:    postgresContainer,
		connString:   connStr,
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
