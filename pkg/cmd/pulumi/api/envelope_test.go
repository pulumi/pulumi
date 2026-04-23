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
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

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

func TestWriteErrorEnvelope_CompactIsOneLine(t *testing.T) {
	t.Parallel()
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrNoMatch, "nope").
		WithSuggestions("try ls")

	var buf bytes.Buffer
	require.NoError(t, WriteErrorEnvelope(&buf, apiErr, false))
	out := buf.String()

	// Exactly one newline at the end; no intermediate newlines.
	assert.True(t, strings.HasSuffix(out, "\n"), "output should end with newline")
	assert.Equal(t, 1, strings.Count(out, "\n"), "compact output should be one line")
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, ErrNoMatch, env.Error.Code)
	assert.Equal(t, "try ls", env.Error.Suggestions[0])
}

func TestWriteErrorEnvelope_PrettyIsIndented(t *testing.T) {
	t.Parallel()
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrNoMatch, "nope")

	var buf bytes.Buffer
	require.NoError(t, WriteErrorEnvelope(&buf, apiErr, true))
	out := buf.String()
	assert.Contains(t, out, "\n  ", "pretty output should use 2-space indent")
}

func TestNewEvent_CarriesSchemaVersionAndTimestamp(t *testing.T) {
	t.Parallel()
	old := now
	defer func() { now = old }()
	now = func() string { return "2026-04-16T10:00:00Z" }

	ev := NewEvent("page")
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

// captureStderr mirrors captureStdout in paginate_test.go but for stderr, so
// we can assert runWithEnvelope writes its JSON envelope there.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stderr
	os.Stderr = w
	done := make(chan []byte, 1)
	go func() {
		b, _ := readAll(r)
		done <- b
	}()
	fn()
	require.NoError(t, w.Close())
	os.Stderr = old
	return string(<-done)
}

// readAll avoids importing io here; io is already imported in paginate_test.
func readAll(r *os.File) ([]byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf.Bytes(), nil
			}
			return buf.Bytes(), err
		}
	}
}

// TestRunWithEnvelope_ReturnsErrorNotExit pins the cleanup contract: the
// wrapper must return the APIError so main.go can run cleanup() (log flush,
// OTel span export, profiling close) before the process exits.
//
//nolint:paralleltest // mutates os.Stderr
func TestRunWithEnvelope_ReturnsErrorNotExit(t *testing.T) {
	apiErr := NewAPIError(cmdutil.ExitCodeError, ErrNoMatch, "no match")
	wrapped := runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return apiErr
	})

	var got error
	stderr := captureStderr(t, func() {
		got = wrapped(&cobra.Command{}, nil)
	})

	// Contract: error is returned, not swallowed by os.Exit.
	require.Error(t, got, "wrapper must return the error, not call os.Exit")
	var returned *APIError
	require.True(t, errors.As(got, &returned))
	assert.Equal(t, cmdutil.ExitCodeError, returned.ExitCode)
	// Envelope was written, and Silent is flipped so DisplayErrorMessage won't duplicate.
	assert.True(t, returned.Silent, "runWithEnvelope must mark the error Silent after writing the envelope")
	assert.Contains(t, stderr, ErrNoMatch, "envelope must carry the structured error code")
}

// TestRunWithEnvelope_WrapsGenericError pins that non-APIError returns get
// classified as cmdutil.ExitInternalError (255) with an envelope.
//
//nolint:paralleltest // mutates os.Stderr
func TestRunWithEnvelope_WrapsGenericError(t *testing.T) {
	raw := errors.New("boom")
	wrapped := runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return raw
	})

	var got error
	_ = captureStderr(t, func() {
		got = wrapped(&cobra.Command{}, nil)
	})

	var apiErr *APIError
	require.True(t, errors.As(got, &apiErr))
	assert.Equal(t, cmdutil.ExitInternalError, apiErr.ExitCode)
	assert.True(t, apiErr.Silent)
}
