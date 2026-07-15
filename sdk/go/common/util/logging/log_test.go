// Copyright 2016, Pulumi Corporation.
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

package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogging(t *testing.T) {
	t.Parallel()

	// Just ensure we can initialize logging (and reset it afterwards).
	prevLog := LogToStderr
	prevV := Verbose
	prevFlow := LogFlow
	InitLogging(true, 9, true)
	InitLogging(prevLog, prevV, prevFlow)
	assert.Equal(t, prevLog, LogToStderr)
	assert.Equal(t, prevV, Verbose)
	assert.Equal(t, prevFlow, LogFlow)
}

// TestInitLoggingIgnoresLogToStderrWithOTel verifies that --logtostderr is
// ignored when logs are exported over OTel: records already reach the engine
// through the OTLP export handler, so writing JSON to stderr would only leak
// them into the engine's display of our output.
func TestInitLoggingIgnoresLogToStderrWithOTel(t *testing.T) {
	t.Setenv("PULUMI_LOG_OTLP_ENDPOINT", "127.0.0.1:1")

	prevLog, prevV, prevFlow := LogToStderr, Verbose, LogFlow
	t.Cleanup(func() {
		shutdownExportHandler()
		handlerMu.Lock()
		primary = discardHandler{}
		rebuildLogger()
		handlerMu.Unlock()
		LogToStderr, Verbose, LogFlow = prevLog, prevV, prevFlow
	})

	InitLogging(true, 0, false)

	handlerMu.RLock()
	defer handlerMu.RUnlock()
	require.NotNil(t, exportHandler)
	assert.Equal(t, discardHandler{}, primary)
}

// TestInitLoggingSkipsLogFileWithOTel verifies that no local log file is
// created when logs are exported over OTel: the engine receives the records
// and writes them to its own log output.
func TestInitLoggingSkipsLogFileWithOTel(t *testing.T) { //nolint:paralleltest // mutates global logging state
	t.Setenv("PULUMI_LOG_OTLP_ENDPOINT", "127.0.0.1:1")

	prevLog, prevV, prevFlow := LogToStderr, Verbose, LogFlow
	prevPath, prevFile := logFilePath, logFile
	t.Cleanup(func() {
		shutdownExportHandler()
		handlerMu.Lock()
		primary = discardHandler{}
		logFilePath, logFile = prevPath, prevFile
		rebuildLogger()
		handlerMu.Unlock()
		LogToStderr, Verbose, LogFlow = prevLog, prevV, prevFlow
	})

	InitLogging(false, 9, false)

	handlerMu.RLock()
	defer handlerMu.RUnlock()
	require.NotNil(t, exportHandler)
	assert.Equal(t, discardHandler{}, primary)
	assert.Equal(t, prevPath, logFilePath)
}

func TestFilter(t *testing.T) {
	t.Parallel()

	filter1 := CreateFilter([]string{"secret1", "secret2"}, "[secret]")
	msg1 := filter1.Filter(
		"These are my secrets: secret1, secret2, secret3, secret10")
	assert.Equal(t,
		"These are my secrets: [secret], [secret], secret3, [secret]0",
		msg1)

	// Ensure that special characters don't screw up the search
	filter2 := CreateFilter([]string{"secret.*", "secre[t]3"}, "[creds]")
	msg2 := filter2.Filter(
		"These are my secrets: secret1, secret2, secret3, secret.*, secre[t]3")
	assert.Equal(t,
		"These are my secrets: secret1, secret2, secret3, [creds], [creds]",
		msg2)

	// Ensure that non-UTF8 characters don't screw up the search
	filter3 := CreateFilter([]string{"nonutf8\xa7", "secret1"}, "[creds]")
	msg3 := filter3.Filter(
		"These are my secrets: secret1, nonutf8\xa7")
	assert.Equal(t,
		"These are my secrets: [creds], [creds]",
		msg3)

	// Short secrets of 1-2 characters are not masked
	filter4 := CreateFilter([]string{"a", "my", "123"}, "[creds]")
	msg4 := filter4.Filter(
		"These are my secrets: a, my, 123")
	assert.Equal(t,
		"These are my secrets: a, my, [creds]",
		msg4)

	// Ensure that multi-line secrets are masked in output.
	filter5 := CreateFilter([]string{"multi\nline\nsecret"}, "[secret]")
	msg5 := filter5.Filter(
		`These are my secrets: multi\nline\nsecret`)
	assert.Equal(t,
		"These are my secrets: [secret]",
		msg5)

	// Ensure that secrets with tabs are masked in output.
	filter6 := CreateFilter([]string{"secretwith\t"}, "[secret]")
	msg6 := filter6.Filter(
		`These are my secrets: secretwith\t`)
	assert.Equal(t,
		"These are my secrets: [secret]",
		msg6)

	// Boolean strings "true" and "false" are not masked, regardless of case.
	filter7 := CreateFilter([]string{"true", "false", "True", "FALSE", "realsecret"}, "[secret]")
	msg7 := filter7.Filter(
		"value is True and FALSE but realsecret is hidden")
	assert.Equal(t,
		"value is True and FALSE but [secret] is hidden",
		msg7)
}

func TestLoggingDoesNotConflictWithGlog(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	require.NoError(t, err)

	// Keep the copied module at a fixed depth below this package so the fixture's
	// relative replace directive points back to the local sdk module.
	dir, err := os.MkdirTemp(wd, ".tmp-glog-flag-conflict-*") //nolint:usetesting
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	fixture := filepath.Join(wd, "testdata", "glog-flag-conflict")
	require.NoError(t, os.CopyFS(dir, os.DirFS(fixture)))

	cmd := exec.Command("go", "run", "-mod=mod", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

// TestInitLoggingFiltersVerbosity verifies that the stderr handler filters records logged directly
// through slog by the -v level: without -v only warnings and errors pass.
//
//nolint:paralleltest // mutates global logging state
func TestInitLoggingFiltersVerbosity(t *testing.T) {
	prevLog, prevV, prevFlow := LogToStderr, Verbose, LogFlow
	t.Cleanup(func() {
		handlerMu.Lock()
		primary = discardHandler{}
		rebuildLogger()
		handlerMu.Unlock()
		LogToStderr, Verbose, LogFlow = prevLog, prevV, prevFlow
	})

	enabled := func(level slog.Level) bool {
		handlerMu.RLock()
		defer handlerMu.RUnlock()
		return primary.Enabled(t.Context(), level)
	}

	InitLogging(true, 0, false)
	assert.False(t, enabled(slog.LevelInfo))
	assert.False(t, enabled(slog.LevelDebug))
	assert.True(t, enabled(slog.LevelWarn))
	assert.True(t, enabled(slog.LevelError))

	InitLogging(true, 1, false)
	assert.True(t, enabled(slog.LevelInfo))
	assert.False(t, enabled(slog.LevelDebug))

	InitLogging(true, 10, false)
	assert.True(t, enabled(slog.LevelDebug))
	assert.False(t, enabled(LevelTrace))

	InitLogging(true, 11, false)
	assert.True(t, enabled(LevelTrace))
}

// TestLogToStderrRespectsVerbosity verifies that with --logtostderr and no -v, neither V-guarded
// nor direct slog info records reach stderr — even while a sink handler (as installed for
// encrypted logs) keeps them flowing — and warnings still show.
//
//nolint:paralleltest // mutates global logging state and os.Stderr
func TestLogToStderrRespectsVerbosity(t *testing.T) {
	prevLog, prevV, prevFlow := LogToStderr, Verbose, LogFlow
	t.Cleanup(func() {
		SetSinkHandler(nil)
		handlerMu.Lock()
		primary = discardHandler{}
		rebuildLogger()
		handlerMu.Unlock()
		LogToStderr, Verbose, LogFlow = prevLog, prevV, prevFlow
	})

	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStderr := os.Stderr
	os.Stderr = w
	InitLogging(true, 0, false)
	os.Stderr = oldStderr

	sink := &recordingHandler{}
	SetSinkHandler(sink)

	V(9).Infof("guarded info %d", 42)
	Infof("unguarded info")
	slog.Info("direct info")
	Warningf("warning shows")

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.NotContains(t, string(out), "guarded info")
	assert.NotContains(t, string(out), "unguarded info")
	assert.NotContains(t, string(out), "direct info")
	assert.Contains(t, string(out), "warning shows")
	assert.True(t, sink.saw("guarded info %d"))
}

type recordingHandler struct {
	mu   sync.Mutex
	msgs []string
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, r.Message)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) saw(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Contains(h.msgs, msg)
}

// TestLogToStderrFiltersHigherVerbosity reproduces `pulumi up --logtostderr -v=1` showing v=5
// records: the primary handler must drop records whose "v" attribute exceeds the requested level,
// for both the V() wrapper and direct slog calls that carry the attribute.
//
//nolint:paralleltest // mutates global logging state and os.Stderr
func TestLogToStderrFiltersHigherVerbosity(t *testing.T) {
	prevLog, prevV, prevFlow := LogToStderr, Verbose, LogFlow
	t.Cleanup(func() {
		SetSinkHandler(nil)
		handlerMu.Lock()
		primary = discardHandler{}
		rebuildLogger()
		handlerMu.Unlock()
		LogToStderr, Verbose, LogFlow = prevLog, prevV, prevFlow
	})

	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStderr := os.Stderr
	os.Stderr = w
	InitLogging(true, 1, false)
	os.Stderr = oldStderr

	sink := &recordingHandler{}
	SetSinkHandler(sink)

	V(1).Infof("v1 shows")
	V(5).Infof("v5 hidden")
	slog.Info("direct v5 hidden", "v", 5)
	slog.Info("unleveled info shows")

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Contains(t, string(out), "v1 shows")
	assert.NotContains(t, string(out), "v5 hidden")
	assert.NotContains(t, string(out), "direct v5 hidden")
	assert.Contains(t, string(out), "unleveled info shows")
	assert.True(t, sink.saw("v5 hidden"))
}
