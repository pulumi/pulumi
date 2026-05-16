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

package org

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
)

// orgAuditLogExportCall records a single ExportAuditLogs invocation made by
// the command under test.
type orgAuditLogExportCall struct {
	org  string
	opts client.ExportAuditLogsOptions
}

// mockOrgAuditLogExportClient stubs orgAuditLogExportClient. It returns a
// canned response body and records every call it received.
type mockOrgAuditLogExportClient struct {
	body  string
	err   error
	calls []orgAuditLogExportCall
}

func (m *mockOrgAuditLogExportClient) ExportAuditLogs(
	_ context.Context, org string, opts client.ExportAuditLogsOptions,
) (io.ReadCloser, error) {
	m.calls = append(m.calls, orgAuditLogExportCall{org: org, opts: opts})
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader(m.body)), nil
}

func stubOrgAuditLogExportFactory(c orgAuditLogExportClient, org string) orgAuditLogExportClientFactory {
	return func(_ context.Context, _ string) (orgAuditLogExportClient, string, error) {
		return c, org, nil
	}
}

func failingOrgAuditLogExportFactory(err error) orgAuditLogExportClientFactory {
	return func(_ context.Context, _ string) (orgAuditLogExportClient, string, error) {
		return nil, "", err
	}
}

func TestOrgAuditLogExport_DefaultOutput(t *testing.T) {
	t.Parallel()

	const fixture = "timestamp,user,event\n" +
		"2025-01-02T03:04:05Z,alice,stack.create\n"

	c := &mockOrgAuditLogExportClient{body: fixture}

	var buf bytes.Buffer
	err := runOrgAuditLogExport(t.Context(), &buf,
		stubOrgAuditLogExportFactory(c, "acme"),
		orgAuditLogExportArgs{
			outputFormat: defaultOrgAuditLogExportOutputFormat(),
			format:       "csv",
			eventType:    "stack.create",
			user:         "alice",
			startTime:    "1735000000",
		})
	require.NoError(t, err)

	assert.Equal(t, []orgAuditLogExportCall{
		{
			org: "acme",
			opts: client.ExportAuditLogsOptions{
				Format:    "csv",
				EventType: "stack.create",
				User:      "alice",
				StartTime: "1735000000",
			},
		},
	}, c.calls)
	assert.Equal(t, fixture, buf.String())
}

func TestOrgAuditLogExport_DefaultFormat(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogExportClient{body: "raw"}

	var buf bytes.Buffer
	err := runOrgAuditLogExport(t.Context(), &buf,
		stubOrgAuditLogExportFactory(c, "acme"),
		orgAuditLogExportArgs{outputFormat: defaultOrgAuditLogExportOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, []orgAuditLogExportCall{
		{
			org:  "acme",
			opts: client.ExportAuditLogsOptions{Format: "csv"},
		},
	}, c.calls)
	assert.Equal(t, "raw", buf.String())
}

func TestOrgAuditLogExport_JSONOutput(t *testing.T) {
	t.Parallel()

	const fixture = "timestamp,user,event\n" +
		"2025-01-02T03:04:05Z,alice,stack.create\n"

	c := &mockOrgAuditLogExportClient{body: fixture}

	args := orgAuditLogExportArgs{
		outputFormat: defaultOrgAuditLogExportOutputFormat(),
		format:       "cef",
	}
	require.NoError(t, args.outputFormat.Set("json"))

	var buf bytes.Buffer
	err := runOrgAuditLogExport(t.Context(), &buf,
		stubOrgAuditLogExportFactory(c, "acme"), args)
	require.NoError(t, err)

	assert.Equal(t, []orgAuditLogExportCall{
		{
			org: "acme",
			opts: client.ExportAuditLogsOptions{
				Format: "cef",
			},
		},
	}, c.calls)

	encoded := base64.StdEncoding.EncodeToString([]byte(fixture))
	assert.JSONEq(t, `{
		"format": "cef",
		"data": "`+encoded+`"
	}`, buf.String())
}

func TestOrgAuditLogExport_InvalidFormat(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogExportClient{}
	var buf bytes.Buffer
	err := runOrgAuditLogExport(t.Context(), &buf,
		stubOrgAuditLogExportFactory(c, "acme"),
		orgAuditLogExportArgs{
			outputFormat: defaultOrgAuditLogExportOutputFormat(),
			format:       "xml",
		})
	require.Error(t, err)
	assert.Equal(t,
		`invalid --format value "xml" (must be 'csv' or 'cef')`,
		err.Error())
	assert.Empty(t, c.calls)
	assert.Equal(t, "", buf.String())
}

func TestOrgAuditLogExport_ClientErrorPropagated(t *testing.T) {
	t.Parallel()

	// The `*client.Client` wraps service errors with "exporting audit logs:
	// %w" before they reach runOrgAuditLogExport, so the run function returns
	// the error verbatim. The mock simulates the already-wrapped error to
	// confirm it isn't re-wrapped.
	c := &mockOrgAuditLogExportClient{
		err: errors.New("exporting audit logs: boom"),
	}

	var buf bytes.Buffer
	err := runOrgAuditLogExport(t.Context(), &buf,
		stubOrgAuditLogExportFactory(c, "acme"),
		orgAuditLogExportArgs{
			outputFormat: defaultOrgAuditLogExportOutputFormat(),
			format:       "csv",
		})
	require.Error(t, err)
	assert.Equal(t, "exporting audit logs: boom", err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgAuditLogExport_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgAuditLogExport(t.Context(), &buf,
		failingOrgAuditLogExportFactory(errors.New("not logged in")),
		orgAuditLogExportArgs{
			outputFormat: defaultOrgAuditLogExportOutputFormat(),
			format:       "csv",
		})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
	assert.Equal(t, "", buf.String())
}
