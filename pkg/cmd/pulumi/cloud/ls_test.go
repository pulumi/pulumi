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

package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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

	m, err := resolveOutput("table")
	require.NoError(t, err)
	assert.Equal(t, outputTable, m)
}

// TestRunLs_RejectsRawAndMarkdown pins that ls refuses --format=raw and
// --format=markdown up front, rather than silently rendering the table.
// These modes are accepted globally by resolveOutput for describe and the
// raw dispatcher.
func TestRunLs_RejectsRawAndMarkdown(t *testing.T) {
	t.Parallel()
	for _, out := range []string{"raw", "markdown", "md"} {
		t.Run(out, func(t *testing.T) {
			t.Parallel()
			err := runLs(t.Context(), io.Discard, io.Discard, out, true, false, false)
			require.Error(t, err)
			var apiErr *APIError
			require.True(t, errors.As(err, &apiErr))
			assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
			assert.Equal(t, cmdutil.ExitConfigurationError, apiErr.ExitCode)
			assert.Equal(t, "format", apiErr.Envelope.Error.Field)
		})
	}
}

// TestEmitLsTable_OutputShape pins the table header row and the trailing
// "N operations." summary line.
func TestEmitLsTable_OutputShape(t *testing.T) {
	t.Parallel()
	idx := loadTestIndex(t)
	view := filterListedOps(idx, true, false)
	var buf bytes.Buffer
	require.NoError(t, emitLsTable(&buf, view))
	out := buf.String()
	assert.Contains(t, out, "TAG")
	assert.Contains(t, out, "METHOD")
	assert.Contains(t, out, "PATH")
	assert.Contains(t, out, "SUMMARY")
	assert.Regexp(t, `\n\d+ operations\.`, out)
}

// TestEmitLsJSON_EnvelopeShape pins the stable JSON envelope fields expected
// by agent consumers: schemaVersion, count, operations[].
func TestEmitLsJSON_EnvelopeShape(t *testing.T) {
	t.Parallel()
	idx := loadTestIndex(t)
	view := filterListedOps(idx, true, false)
	var buf bytes.Buffer
	require.NoError(t, emitLsJSON(&buf, view))
	var env struct {
		SchemaVersion int `json:"schemaVersion"`
		Count         int `json:"count"`
		Operations    []struct {
			Method      string `json:"method"`
			Path        string `json:"path"`
			OperationID string `json:"operationId"`
		} `json:"operations"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &env))
	assert.Equal(t, SchemaVersion, env.SchemaVersion)
	assert.Equal(t, len(view.Operations), env.Count)
	require.NotEmpty(t, env.Operations)
	assert.NotEmpty(t, env.Operations[0].Method)
	assert.NotEmpty(t, env.Operations[0].Path)
}
