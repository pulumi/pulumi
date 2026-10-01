// Copyright 2026, Pulumi Corporation.
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

package diy

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	_ "github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres" // driver for postgres://
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	stackpkg "github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/tests/integration/backend/diy/pgtest"
)

// checkpointFormatFixture owns the Postgres container and a project/stack name
// used across the checkpoint_format tests.
type checkpointFormatFixture struct {
	pg        *pgtest.Database
	tableName string
	project   workspace.Project
	stackName string
}

func newCheckpointFormatFixture(t *testing.T) *checkpointFormatFixture {
	t.Helper()
	skipPostgresTestIfNeeded(t)
	desc := "Test project for checkpoint_format"
	return &checkpointFormatFixture{
		pg:        pgtest.New(t),
		tableName: "pulumi_ckpt_" + pgtest.GenerateID(),
		project: workspace.Project{
			Name:        "test-ckpt-project",
			Runtime:     workspace.NewProjectRuntimeInfo("nodejs", nil),
			Description: &desc,
		},
		stackName: "ckpt-stack-" + pgtest.GenerateID(),
	}
}

// urlWithFormat builds the backend URL, optionally setting checkpoint_format.
// Passing "" leaves the flag off (legacy default).
func (f *checkpointFormatFixture) urlWithFormat(format string) string {
	u := f.pg.ConnectionStringWithTable(f.tableName)
	if format != "" {
		u += "&checkpoint_format=" + format
	}
	return u
}

// openDB opens a raw sql.DB against the test Postgres for direct assertions.
func (f *checkpointFormatFixture) openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", f.pg.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// writeCheckpoint creates a stack and imports a snapshot that contains a secret
// output. The returned sentinel is the plaintext that the secret wraps — it
// must never appear in the stored row.
func (f *checkpointFormatFixture) writeCheckpoint(t *testing.T, url, sentinel string) {
	t.Helper()
	ctx := t.Context()

	b, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), url, nil)
	require.NoError(t, err)
	b.SetCurrentProject(&f.project)

	stackRef, err := b.ParseStackReference(f.stackName)
	require.NoError(t, err)
	stack, err := b.CreateStack(ctx, stackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, stack)

	// Passphrase-encrypt the sentinel so the serialized checkpoint contains
	// ciphertext rather than plaintext.
	_, sm, err := passphrase.NewPassphraseSecretsManager("testpassphrase")
	require.NoError(t, err)

	secretProp := resource.MakeSecret(resource.NewProperty(sentinel))
	urn := resource.NewURN(
		tokens.QName(f.stackName), f.project.Name, "", "test:index:Resource", "secret-resource")

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			Type:    resource.RootStackType,
			URN:     resource.CreateURN(string(tokens.QName(f.stackName)), string(resource.RootStackType), "", string(f.project.Name), f.stackName),
			Outputs: resource.PropertyMap{},
		},
		{
			Type:    "test:index:Resource",
			URN:     urn,
			Custom:  true,
			ID:      "secret-resource-id",
			Inputs:  resource.PropertyMap{"name": resource.NewProperty("secret-resource")},
			Outputs: resource.PropertyMap{"secret": secretProp},
		},
	}, nil, deploy.SnapshotMetadata{})

	untyped, err := stackpkg.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)
	require.NoError(t, b.ImportDeployment(ctx, stack, untyped))
}

// TestPostgresBackendCheckpointFormatLegacy verifies the default flag keeps
// all rows in the legacy base64-wrapped-JSON column.
func TestPostgresBackendCheckpointFormatLegacy(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)
	f.writeCheckpoint(t, f.urlWithFormat(""), "SENTINEL-legacy")

	db := f.openDB(t)
	var jsonbCount int
	//nolint:gosec // tableName is test-controlled
	require.NoError(t, db.QueryRowContext(t.Context(),
		fmt.Sprintf("SELECT count(*) FROM %s WHERE data_jsonb IS NOT NULL", f.tableName),
	).Scan(&jsonbCount))
	assert.Zero(t, jsonbCount, "no rows should use data_jsonb under the default flag")

	var legacyCount int
	//nolint:gosec // tableName is test-controlled
	require.NoError(t, db.QueryRowContext(t.Context(),
		fmt.Sprintf("SELECT count(*) FROM %s WHERE data IS NOT NULL", f.tableName),
	).Scan(&legacyCount))
	assert.Positive(t, legacyCount, "at least one row should have data populated")
}

// TestPostgresBackendCheckpointFormatJSONB verifies that with the flag on,
// only .pulumi/stacks/*.json keys land in data_jsonb; everything else stays
// in the legacy column.
func TestPostgresBackendCheckpointFormatJSONB(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)
	f.writeCheckpoint(t, f.urlWithFormat("jsonb"), "SENTINEL-jsonb")

	db := f.openDB(t)

	// The primary checkpoint lands in data_jsonb with data NULL. The DIY
	// backend prefixes keys with the URL path (database name), so we match
	// ".pulumi/stacks/" anywhere in the key.
	var ckptJSONB, ckptLegacy int
	//nolint:gosec // tableName is test-controlled
	q := fmt.Sprintf(
		"SELECT "+
			"count(*) FILTER (WHERE data_jsonb IS NOT NULL AND data IS NULL), "+
			"count(*) FILTER (WHERE data IS NOT NULL) "+
			"FROM %s WHERE key LIKE '%%.pulumi/stacks/%%' AND key LIKE '%%.json'",
		f.tableName)
	require.NoError(t, db.QueryRowContext(t.Context(), q).Scan(&ckptJSONB, &ckptLegacy))
	assert.Positive(t, ckptJSONB, "expected at least one checkpoint row in data_jsonb")
	assert.Zero(t, ckptLegacy, "no .pulumi/stacks/<...>.json row should still be in data")

	// Every row in data_jsonb must live under `.pulumi/` — meaning it's a
	// checkpoint, history entry, backup, lock, or Copy-derived sibling like
	// `.json.bak` that inherited format from its jsonb source row. A row
	// outside `.pulumi/` in data_jsonb would indicate the write-path gate
	// leaked to a non-state blob.
	var leaked int
	//nolint:gosec // tableName is test-controlled
	q = fmt.Sprintf(
		"SELECT count(*) FROM %s WHERE data_jsonb IS NOT NULL AND key NOT LIKE '%%.pulumi/%%'",
		f.tableName)
	require.NoError(t, db.QueryRowContext(t.Context(), q).Scan(&leaked))
	assert.Zero(t, leaked, "only keys under `.pulumi/` should use data_jsonb")
}

// TestPostgresBackendCheckpointQueryable verifies the stored JSONB actually
// lets you query into the deployment tree with plain SQL. Uses a structure-
// agnostic jsonpath so the test survives minor checkpoint shape tweaks.
func TestPostgresBackendCheckpointQueryable(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)
	f.writeCheckpoint(t, f.urlWithFormat("jsonb"), "SENTINEL-queryable")

	db := f.openDB(t)

	// The checkpoint tree contains a "resources" array somewhere — exercise
	// that the GIN index / jsonpath operators work against it.
	var hasResources bool
	//nolint:gosec // tableName is test-controlled
	q := fmt.Sprintf(
		"SELECT bool_or(data_jsonb @? '$.**.resources[*]') FROM %s "+
			"WHERE key LIKE '%%.pulumi/stacks/%%' AND key LIKE '%%.json'",
		f.tableName)
	require.NoError(t, db.QueryRowContext(t.Context(), q).Scan(&hasResources))
	assert.True(t, hasResources, "expected a resources array somewhere in data_jsonb")
}

// TestPostgresBackendSecretsStayEncrypted is the load-bearing safety test:
// enabling JSONB storage must not expose any plaintext secret that was
// encrypted by the passphrase secrets manager.
func TestPostgresBackendSecretsStayEncrypted(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)
	const sentinel = "SUPERSECRETSENTINEL"
	f.writeCheckpoint(t, f.urlWithFormat("jsonb"), sentinel)

	db := f.openDB(t)
	var raw sql.NullString
	//nolint:gosec // tableName is test-controlled
	q := fmt.Sprintf(
		"SELECT string_agg(data_jsonb::text, '\n') FROM %s "+
			"WHERE key LIKE '%%.pulumi/stacks/%%' AND key LIKE '%%.json'",
		f.tableName)
	require.NoError(t, db.QueryRowContext(t.Context(), q).Scan(&raw))
	require.True(t, raw.Valid, "expected at least one checkpoint row")
	assert.NotContains(t, raw.String, sentinel,
		"plaintext secret leaked into data_jsonb — passphrase encryption bypassed")
}

// TestPostgresBackendMixedFormatReads hand-seeds one legacy-formatted row
// alongside the jsonb row written via the backend, and verifies both resolve
// through the backend's read path.
func TestPostgresBackendMixedFormatReads(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)

	// First, write a stack using the jsonb flag — this creates the table and
	// a full set of rows in the new format.
	f.writeCheckpoint(t, f.urlWithFormat("jsonb"), "SENTINEL-mixed")

	// Now hand-seed a second stack's meta.yaml entry in the legacy format so
	// the table contains a deliberate mix of both shapes.
	db := f.openDB(t)
	legacyKey := ".pulumi/meta-mixed-" + pgtest.GenerateID() + ".yaml"
	wrapper, err := json.Marshal(map[string]string{
		"data": base64.StdEncoding.EncodeToString([]byte("legacy-hand-seeded-payload")),
	})
	require.NoError(t, err)
	//nolint:gosec // tableName is test-controlled
	_, err = db.ExecContext(t.Context(),
		fmt.Sprintf("INSERT INTO %s (key, data, data_jsonb) VALUES ($1, $2::json, NULL)", f.tableName),
		legacyKey, string(wrapper))
	require.NoError(t, err)

	// Opening a backend with the flag *off* must still enumerate both rows
	// (the jsonb-formatted stack is discoverable via ListStacks).
	b, err := diy.New(t.Context(), diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), f.urlWithFormat(""), nil)
	require.NoError(t, err)
	b.SetCurrentProject(&f.project)
	stacks, _, err := b.ListStacks(t.Context(), backend.ListStacksFilter{}, nil)
	require.NoError(t, err)
	assert.Len(t, stacks, 1, "the jsonb-stored stack must be visible under flag=off")
}

// TestPostgresBackendFlagToggleReadback writes with the flag on, reconnects
// with the flag off (and vice versa), and confirms both orderings can still
// read the stack back out.
func TestPostgresBackendFlagToggleReadback(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)
	f.writeCheckpoint(t, f.urlWithFormat("jsonb"), "SENTINEL-toggle")

	// Reconnect with flag off and export the stack.
	ctx := t.Context()
	b, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), f.urlWithFormat(""), nil)
	require.NoError(t, err)
	b.SetCurrentProject(&f.project)

	ref, err := b.ParseStackReference(f.stackName)
	require.NoError(t, err)
	stack, err := b.GetStack(ctx, ref)
	require.NoError(t, err)
	require.NotNil(t, stack, "stack written under flag=jsonb should be readable under flag=off")

	exported, err := b.ExportDeployment(ctx, stack)
	require.NoError(t, err)
	require.NotEmpty(t, exported.Deployment)
}

// TestPostgresBackendRejectInvalidCheckpointFormat confirms that an unknown
// checkpoint_format value fails at backend initialization with a clear error.
func TestPostgresBackendRejectInvalidCheckpointFormat(t *testing.T) {
	t.Parallel()
	f := newCheckpointFormatFixture(t)
	_, err := diy.New(t.Context(), diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	}), f.urlWithFormat("bogus"), nil)
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "checkpoint_format") && strings.Contains(err.Error(), "bogus"),
		"expected error to mention checkpoint_format and the bad value, got: %v", err)
}
