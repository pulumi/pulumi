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
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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

// withFilters runs fn with the given global filters installed, then restores
// the previous filter list. Not parallel-safe — callers must avoid t.Parallel().
func withFilters(t *testing.T, newFilters []Filter, fn func()) {
	t.Helper()
	rwLock.Lock()
	prev := filters
	filters = newFilters
	rwLock.Unlock()
	t.Cleanup(func() {
		rwLock.Lock()
		filters = prev
		rwLock.Unlock()
	})
	fn()
}

// recordToMap decodes a single JSON line emitted by slog.NewJSONHandler into
// a map for easy assertion.
func recordToMap(t *testing.T, line []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(line, &m))
	return m
}

func TestFilteringHandlerFiltersMessage(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: LevelTrace})
	logger := slog.New(filteringHandler{inner: inner})

	withFilters(t, []Filter{CreateFilter([]string{"hunter2"}, "[secret]")}, func() {
		logger.Info("the password is hunter2 and the user is alice")
	})

	rec := recordToMap(t, buf.Bytes())
	assert.Equal(t, "the password is [secret] and the user is alice", rec["msg"])
}

func TestFilteringHandlerFiltersStringAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: LevelTrace})
	logger := slog.New(filteringHandler{inner: inner})

	withFilters(t, []Filter{CreateFilter([]string{"hunter2"}, "[secret]")}, func() {
		logger.Info("login", "password", "hunter2", "user", "alice")
	})

	rec := recordToMap(t, buf.Bytes())
	assert.Equal(t, "[secret]", rec["password"])
	assert.Equal(t, "alice", rec["user"])
}

func TestFilteringHandlerLeavesNonStringAttrsAlone(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: LevelTrace})
	logger := slog.New(filteringHandler{inner: inner})

	withFilters(t, []Filter{CreateFilter([]string{"hunter2"}, "[secret]")}, func() {
		logger.Info("login", "attempts", 3, "ok", true)
	})

	rec := recordToMap(t, buf.Bytes())
	assert.Equal(t, float64(3), rec["attempts"])
	assert.Equal(t, true, rec["ok"])
}

func TestFilteringHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: LevelTrace})
	logger := slog.New(filteringHandler{inner: inner})

	withFilters(t, []Filter{CreateFilter([]string{"hunter2"}, "[secret]")}, func() {
		// WithAttrs eagerly filters secret-bearing attributes too.
		sub := logger.With("password", "hunter2", "service", "auth")
		sub.Info("login")
	})

	rec := recordToMap(t, buf.Bytes())
	assert.Equal(t, "[secret]", rec["password"])
	assert.Equal(t, "auth", rec["service"])
}

func TestFilteringHandlerNoFilters(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: LevelTrace})
	logger := slog.New(filteringHandler{inner: inner})

	withFilters(t, nil, func() {
		logger.Info("hello", "key", "value")
	})

	rec := recordToMap(t, buf.Bytes())
	assert.Equal(t, "hello", rec["msg"])
	assert.Equal(t, "value", rec["key"])
}

func TestFilteringHandlerEnabledForwarded(t *testing.T) {
	t.Parallel()
	inner := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := filteringHandler{inner: inner}

	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, h.Enabled(context.Background(), slog.LevelError))
}
