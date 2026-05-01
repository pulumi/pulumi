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
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIError_FluentBuilders(t *testing.T) {
	t.Parallel()
	err := NewAPIError(cmdutil.ExitCodeError, ErrMissingContext, "missing var").
		WithField("orgName").
		WithSuggestions("pass --org <name>").
		WithHTTP(404, map[string]string{"err": "not found"})

	assert.Equal(t, cmdutil.ExitCodeError, err.ExitCode)
	assert.Equal(t, "orgName", err.Envelope.Error.Field)
	assert.Equal(t, []string{"pass --org <name>"}, err.Envelope.Error.Suggestions)
	assert.Equal(t, 404, err.Envelope.Error.HTTPStatus)
	assert.Equal(t, "missing var", err.Error())
	assert.Equal(t, SchemaVersion, err.Envelope.Error.SchemaVersion)
}

func TestWriteErrorEnvelope_NonInteractiveIsCompactJSON(t *testing.T) {
	t.Parallel()
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrNoMatch, "nope").
		WithSuggestions("try list")

	var buf bytes.Buffer
	require.NoError(t, WriteErrorEnvelope(&buf, apiErr, false))
	out := buf.String()

	// Single-line JSON for log-line parsers and agents.
	assert.True(t, strings.HasSuffix(out, "\n"), "output should end with newline")
	assert.Equal(t, 1, strings.Count(out, "\n"), "non-interactive output should be one line")
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, ErrNoMatch, env.Error.Code)
	assert.Equal(t, "try list", env.Error.Suggestions[0])
}

func TestWriteErrorEnvelope_InteractiveIsHumanReadable(t *testing.T) {
	t.Parallel()
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags, "--format=yaml is not supported").
		WithField("format").
		WithSuggestions("--format=json", "--format=table")

	var buf bytes.Buffer
	require.NoError(t, WriteErrorEnvelope(&buf, apiErr, true))
	out := buf.String()

	// Plain text — must NOT be JSON.
	assert.False(t, strings.HasPrefix(strings.TrimSpace(out), "{"),
		"interactive output should be human text, not JSON")
	assert.Contains(t, out, "error: --format=yaml is not supported")
	assert.Contains(t, out, "field: format")
	assert.Contains(t, out, "Suggestions:")
	assert.Contains(t, out, "- --format=json")
	assert.Contains(t, out, "- --format=table")
}

func TestWriteErrorEnvelope_InteractiveHTTPStatus(t *testing.T) {
	t.Parallel()
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrHTTP4xx, "not found").
		WithHTTP(404, nil)

	var buf bytes.Buffer
	require.NoError(t, WriteErrorEnvelope(&buf, apiErr, true))
	assert.Contains(t, buf.String(), "HTTP 404")
}

func TestNewEvent_CarriesSchemaVersionAndTimestamp(t *testing.T) {
	t.Parallel()
	fixedTime, err := time.Parse(time.RFC3339, "2026-04-16T10:00:00Z")
	require.NoError(t, err)

	ev := NewEvent("page", fixedTime)
	ev.Page = 3
	ev.Count = 42

	var buf bytes.Buffer
	require.NoError(t, WriteEvent(&buf, ev))
	var decoded Event
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))

	assert.Equal(t, SchemaVersion, decoded.SchemaVersion)
	assert.Equal(t, "page", decoded.Event)
	assert.Equal(t, "2026-04-16T10:00:00Z", decoded.Timestamp)
	assert.Equal(t, 3, decoded.Page)
	assert.Equal(t, 42, decoded.Count)
}

func TestWriteJSON_RoundTrip(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"schemaVersion": SchemaVersion,
		"nested":        map[string]string{"k": "v"},
	}
	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, input, false))
	assert.True(t, strings.HasSuffix(buf.String(), "\n"))

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.EqualValues(t, SchemaVersion, out["schemaVersion"])
}

// TestRunWithEnvelope_ReturnsErrorNotExit pins the cleanup contract: the
// wrapper must return the APIError so main.go can run cleanup() (log flush,
// OTel span export, profiling close) before the process exits.
func TestRunWithEnvelope_ReturnsErrorNotExit(t *testing.T) {
	t.Parallel()
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrNoMatch, "no match")
	wrapped := runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return apiErr
	})

	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	got := wrapped(cmd, nil)

	// Contract: error is returned, not swallowed by os.Exit.
	require.Error(t, got, "wrapper must return the error, not call os.Exit")
	var returned *APIError
	require.True(t, errors.As(got, &returned))
	assert.Equal(t, cmdutil.ExitCodeError, returned.ExitCode)
	// Envelope was written, and Silent is flipped so DisplayErrorMessage won't duplicate.
	assert.True(t, returned.Silent, "runWithEnvelope must mark the error Silent after writing the envelope")
	assert.Contains(t, stderr.String(), ErrNoMatch, "envelope must carry the structured error code")
}

// TestRunWithEnvelope_WrapsGenericError pins that non-APIError returns get
// classified as cmdutil.ExitInternalError (255) with an envelope.
func TestRunWithEnvelope_WrapsGenericError(t *testing.T) {
	t.Parallel()
	raw := errors.New("boom")
	wrapped := runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return raw
	})

	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	got := wrapped(cmd, nil)

	var apiErr *APIError
	require.True(t, errors.As(got, &apiErr))
	assert.Equal(t, cmdutil.ExitInternalError, apiErr.ExitCode)
	assert.True(t, apiErr.Silent)
}
