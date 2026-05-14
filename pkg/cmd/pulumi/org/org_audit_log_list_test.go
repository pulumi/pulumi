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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// orgAuditLogListCall records a single ListAuditLogs invocation made by the
// command under test.
type orgAuditLogListCall struct {
	org  string
	opts client.ListAuditLogsOptions
}

// mockOrgAuditLogListClient stubs orgAuditLogListClient. It returns a canned
// response and records every call it received.
type mockOrgAuditLogListClient struct {
	resp  apitype.ListAuditLogEventsResponse
	err   error
	calls []orgAuditLogListCall
}

func (m *mockOrgAuditLogListClient) ListAuditLogs(
	_ context.Context, org string, opts client.ListAuditLogsOptions,
) (apitype.ListAuditLogEventsResponse, error) {
	m.calls = append(m.calls, orgAuditLogListCall{org: org, opts: opts})
	return m.resp, m.err
}

func stubOrgAuditLogListFactory(c orgAuditLogListClient, org string) orgAuditLogListClientFactory {
	return func(_ context.Context, _ string) (orgAuditLogListClient, string, error) {
		return c, org, nil
	}
}

func failingOrgAuditLogListFactory(err error) orgAuditLogListClientFactory {
	return func(_ context.Context, _ string) (orgAuditLogListClient, string, error) {
		return nil, "", err
	}
}

func sampleAuditLogEvent() apitype.AuditLogEvent {
	return apitype.AuditLogEvent{
		// 2025-01-02T03:04:05Z
		Timestamp:   1735787045,
		Name:        "stack.create",
		SourceIP:    "203.0.113.7",
		Description: "Created stack acme/web/prod",
		User: apitype.UserInfo{
			Name:        "Alice Example",
			GitHubLogin: "alice",
		},
		Event: "stack",
	}
}

func TestOrgAuditLogList_DefaultOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{
			AuditLogEvents: []apitype.AuditLogEvent{sampleAuditLogEvent()},
		},
	}

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{
			eventType: "stack.create",
			user:      "alice",
			startTime: "1735000000",
		})
	require.NoError(t, err)

	assert.Equal(t, []orgAuditLogListCall{
		{
			org: "acme",
			opts: client.ListAuditLogsOptions{
				EventType: "stack.create",
				User:      "alice",
				StartTime: "1735000000",
			},
		},
	}, c.calls)

	expected := "" +
		"┌──────────────────────┬───────┬───────┬──────────────┬─────────────┐\n" +
		"│ TIMESTAMP            │ USER  │ EVENT │ NAME         │ SOURCE IP   │\n" +
		"├──────────────────────┼───────┼───────┼──────────────┼─────────────┤\n" +
		"│ 2025-01-02T03:04:05Z │ alice │ stack │ stack.create │ 203.0.113.7 │\n" +
		"└──────────────────────┴───────┴───────┴──────────────┴─────────────┘\n"
	assert.Equal(t, expected, buf.String())
}

func TestOrgAuditLogList_EmptyText(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{},
	}

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{})
	require.NoError(t, err)
	assert.Equal(t, "No audit log events found.\n", buf.String())
}

func TestOrgAuditLogList_TextContinuationHint(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{
			AuditLogEvents:    []apitype.AuditLogEvent{sampleAuditLogEvent()},
			ContinuationToken: "next-page",
		},
	}

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{})
	require.NoError(t, err)

	expected := "" +
		"┌──────────────────────┬───────┬───────┬──────────────┬─────────────┐\n" +
		"│ TIMESTAMP            │ USER  │ EVENT │ NAME         │ SOURCE IP   │\n" +
		"├──────────────────────┼───────┼───────┼──────────────┼─────────────┤\n" +
		"│ 2025-01-02T03:04:05Z │ alice │ stack │ stack.create │ 203.0.113.7 │\n" +
		"└──────────────────────┴───────┴───────┴──────────────┴─────────────┘\n" +
		"\nMore results available. Re-run with --continuation-token \"next-page\" to continue.\n"
	assert.Equal(t, expected, buf.String())
}

func TestOrgAuditLogList_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{
			AuditLogEvents:    []apitype.AuditLogEvent{sampleAuditLogEvent()},
			ContinuationToken: "next-page",
		},
	}

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{
			continuationToken: "prev-page",
			output:            "json",
		})
	require.NoError(t, err)

	assert.Equal(t, []orgAuditLogListCall{
		{
			org: "acme",
			opts: client.ListAuditLogsOptions{
				ContinuationToken: "prev-page",
			},
		},
	}, c.calls)

	assert.JSONEq(t, `{
		"events": [{
			"timestamp": 1735787045,
			"name": "stack.create",
			"sourceIP": "203.0.113.7",
			"description": "Created stack acme/web/prod",
			"user": {
				"name": "Alice Example",
				"githubLogin": "alice",
				"avatarUrl": ""
			},
			"event": "stack"
		}],
		"continuationToken": "next-page"
	}`, buf.String())
}

func TestOrgAuditLogList_JSONNilSliceNormalized(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{
			AuditLogEvents: nil,
		},
	}

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{output: "json"})
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"events": [],
		"continuationToken": ""
	}`, buf.String())
}

func TestOrgAuditLogList_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{}
	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{output: "yaml"})
	require.Error(t, err)
	assert.Equal(t,
		`invalid --output value "yaml" (must be 'default' or 'json')`,
		err.Error())
	assert.Empty(t, c.calls)
	assert.Equal(t, "", buf.String())
}

func TestOrgAuditLogList_ClientErrorPropagated(t *testing.T) {
	t.Parallel()

	// The `*client.Client` wraps service errors with "listing audit logs: %w"
	// before they reach runOrgAuditLogList, so the run function returns the
	// error verbatim. The mock simulates the already-wrapped error to confirm
	// it isn't re-wrapped.
	c := &mockOrgAuditLogListClient{
		err: errors.New("listing audit logs: boom"),
	}

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"),
		orgAuditLogListArgs{})
	require.Error(t, err)
	assert.Equal(t, "listing audit logs: boom", err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgAuditLogList_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		failingOrgAuditLogListFactory(errors.New("not logged in")),
		orgAuditLogListArgs{})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
	assert.Equal(t, "", buf.String())
}
