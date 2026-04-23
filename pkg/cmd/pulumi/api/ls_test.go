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

package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterListedOps(t *testing.T) {
	t.Parallel()

	idx := &Index{
		Operations: []*Operation{
			{OperationID: "Stable"},
			{OperationID: "Preview", IsPreview: true},
			{OperationID: "Deprecated", IsDeprecated: true},
			{OperationID: "Both", IsPreview: true, IsDeprecated: true},
		},
		ByKey:       map[string]*Operation{},
		SpecVersion: "abc",
	}

	t.Run("default hides deprecated, keeps preview", func(t *testing.T) {
		t.Parallel()
		view := filterListedOps(idx, true, false)
		ids := opIDs(view.Operations)
		assert.ElementsMatch(t, []string{"Stable", "Preview"}, ids)
	})

	t.Run("include-deprecated surfaces deprecated", func(t *testing.T) {
		t.Parallel()
		view := filterListedOps(idx, true, true)
		ids := opIDs(view.Operations)
		assert.ElementsMatch(t, []string{"Stable", "Preview", "Deprecated", "Both"}, ids)
	})

	t.Run("include-preview=false hides preview", func(t *testing.T) {
		t.Parallel()
		view := filterListedOps(idx, false, false)
		ids := opIDs(view.Operations)
		assert.ElementsMatch(t, []string{"Stable"}, ids)
	})

	t.Run("view shares metadata", func(t *testing.T) {
		t.Parallel()
		view := filterListedOps(idx, true, false)
		assert.Equal(t, idx.SpecVersion, view.SpecVersion)
	})
}

func opIDs(ops []*Operation) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, op.OperationID)
	}
	return out
}

func TestResolveOutputAcceptsTable(t *testing.T) {
	t.Parallel()

	m, err := ResolveOutput("table")
	require.NoError(t, err)
	assert.Equal(t, OutputDefault, m)
}

func TestReconcileJQOutput(t *testing.T) {
	t.Parallel()

	t.Run("jq empty returns mode unchanged", func(t *testing.T) {
		t.Parallel()
		m, err := reconcileJQOutput(OutputDefault, "", "", false)
		require.NoError(t, err)
		assert.Equal(t, OutputDefault, m)
	})

	t.Run("jq without output auto-upgrades to JSON", func(t *testing.T) {
		t.Parallel()
		m, err := reconcileJQOutput(OutputDefault, ".x", "", false)
		require.NoError(t, err)
		assert.Equal(t, OutputJSON, m)
	})

	t.Run("jq with explicit non-JSON output errors", func(t *testing.T) {
		t.Parallel()
		_, err := reconcileJQOutput(OutputRaw, ".x", "raw", true)
		require.Error(t, err)
		var apiErr *APIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
		assert.Equal(t, cmdutil.ExitConfigurationError, apiErr.ExitCode)
		assert.Equal(t, "jq", apiErr.Envelope.Error.Field)
	})

	t.Run("jq with explicit JSON output is accepted", func(t *testing.T) {
		t.Parallel()
		m, err := reconcileJQOutput(OutputJSON, ".x", "json", true)
		require.NoError(t, err)
		assert.Equal(t, OutputJSON, m)
	})
}

// TestRunLs_RejectsRawAndMarkdown pins that ls refuses --output=raw and
// --output=markdown up front, rather than silently rendering the table.
// These modes are accepted globally by ResolveOutput for describe and the
// raw dispatcher.
func TestRunLs_RejectsRawAndMarkdown(t *testing.T) {
	t.Parallel()
	for _, out := range []string{"raw", "markdown", "md"} {
		t.Run(out, func(t *testing.T) {
			t.Parallel()
			err := runLs(t.Context(), out, "", true, true, false, false)
			require.Error(t, err)
			var apiErr *APIError
			require.True(t, errors.As(err, &apiErr))
			assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
			assert.Equal(t, cmdutil.ExitConfigurationError, apiErr.ExitCode)
			assert.Equal(t, "output", apiErr.Envelope.Error.Field)
		})
	}
}

// TestEmitLsTable_OutputShape pins the table header row and the trailing
// "N operations." summary line.
//
//nolint:paralleltest // mutates os.Stdout
func TestEmitLsTable_OutputShape(t *testing.T) {
	idx := loadTestIndex(t)
	view := filterListedOps(idx, true, false)
	out := captureStdout(t, func() {
		require.NoError(t, emitLsTable(view))
	})
	assert.Contains(t, out, "TAG")
	assert.Contains(t, out, "METHOD")
	assert.Contains(t, out, "PATH")
	assert.Contains(t, out, "SUMMARY")
	assert.Regexp(t, `\n\d+ operations\.`, out)
}

// TestEmitLsJSON_EnvelopeShape pins the stable JSON envelope fields expected
// by agent consumers: schemaVersion, count, operations[].
//
//nolint:paralleltest // mutates os.Stdout
func TestEmitLsJSON_EnvelopeShape(t *testing.T) {
	idx := loadTestIndex(t)
	view := filterListedOps(idx, true, false)
	out := captureStdout(t, func() {
		require.NoError(t, emitLsJSON(view, ""))
	})
	var env struct {
		SchemaVersion int `json:"schemaVersion"`
		Count         int `json:"count"`
		Operations    []struct {
			Method      string `json:"method"`
			Path        string `json:"path"`
			OperationID string `json:"operationId"`
		} `json:"operations"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &env))
	assert.Equal(t, SchemaVersion, env.SchemaVersion)
	assert.Equal(t, len(view.Operations), env.Count)
	require.NotEmpty(t, env.Operations)
	assert.NotEmpty(t, env.Operations[0].Method)
	assert.NotEmpty(t, env.Operations[0].Path)
}
