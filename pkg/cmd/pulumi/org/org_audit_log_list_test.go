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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// fakeAuditLogListClient stubs orgAuditLogListClient with a dynamic response
// function so tests can drive multi-page scenarios.
type fakeAuditLogListClient struct {
	nextResp func(ctx context.Context, org string, opts client.ListAuditLogsOptions) apitype.ListAuditLogEventsResponse
}

func (f *fakeAuditLogListClient) ListAuditLogs(
	ctx context.Context, org string, opts client.ListAuditLogsOptions,
) (apitype.ListAuditLogEventsResponse, error) {
	return f.nextResp(ctx, org, opts), nil
}

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
		Event:       "stack.create",
		SourceIP:    "203.0.113.7",
		Description: "Created stack acme/web/prod",
		User: apitype.UserInfo{
			Name:        "Alice Example",
			GitHubLogin: "alice",
		},
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
			outputFormat: defaultOrgAuditLogListOutputFormat(),
			eventType:    "stack.create",
			user:         "alice",
			startTime:    "1735000000",
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
		"┌──────────────────────┬───────┬──────────────┬─────────────────────────────┬─────────────┐\n" +
		"│ TIMESTAMP            │ USER  │ EVENT        │ DESCRIPTION                 │ SOURCE IP   │\n" +
		"├──────────────────────┼───────┼──────────────┼─────────────────────────────┼─────────────┤\n" +
		"│ 2025-01-02T03:04:05Z │ alice │ stack.create │ Created stack acme/web/prod │ 203.0.113.7 │\n" +
		"└──────────────────────┴───────┴──────────────┴─────────────────────────────┴─────────────┘\n"
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
		orgAuditLogListArgs{outputFormat: defaultOrgAuditLogListOutputFormat()})
	require.NoError(t, err)
	assert.Equal(t, "No audit log events found.\n", buf.String())
}

func TestOrgAuditLogList_AutoPaginatesWithCount(t *testing.T) {
	t.Parallel()

	// First page has 1 event with a continuation token; second page has another
	// event. --count=2 should drive a second call.
	page2Event := sampleAuditLogEvent()
	page2Event.Event = "stack.delete"

	calls := 0
	c := &fakeAuditLogListClient{
		nextResp: func(_ context.Context, _ string, opts client.ListAuditLogsOptions) apitype.ListAuditLogEventsResponse {
			calls++
			if opts.ContinuationToken == "" {
				return apitype.ListAuditLogEventsResponse{
					AuditLogEvents:    []apitype.AuditLogEvent{sampleAuditLogEvent()},
					ContinuationToken: "tok",
				}
			}
			return apitype.ListAuditLogEventsResponse{
				AuditLogEvents: []apitype.AuditLogEvent{page2Event},
			}
		},
	}

	args := orgAuditLogListArgs{
		outputFormat: defaultOrgAuditLogListOutputFormat(),
		count:        2,
	}
	require.NoError(t, args.outputFormat.Set("json"))

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"), args)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)

	var envelope auditLogListEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.Equal(t, 2, envelope.Count)
}

func TestOrgAuditLogList_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{
			AuditLogEvents: []apitype.AuditLogEvent{sampleAuditLogEvent()},
		},
	}

	args := orgAuditLogListArgs{outputFormat: defaultOrgAuditLogListOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"), args)
	require.NoError(t, err)

	assert.Equal(t, []orgAuditLogListCall{
		{
			org:  "acme",
			opts: client.ListAuditLogsOptions{},
		},
	}, c.calls)

	assert.JSONEq(t, `{
		"events": [{
			"timestamp": 1735787045,
			"event": "stack.create",
			"sourceIP": "203.0.113.7",
			"description": "Created stack acme/web/prod",
			"user": {
				"name": "Alice Example",
				"githubLogin": "alice",
				"avatarUrl": ""
			}
		}],
		"count": 1
	}`, buf.String())
}

func TestOrgAuditLogList_JSONNilSliceNormalized(t *testing.T) {
	t.Parallel()

	c := &mockOrgAuditLogListClient{
		resp: apitype.ListAuditLogEventsResponse{
			AuditLogEvents: nil,
		},
	}

	args := orgAuditLogListArgs{outputFormat: defaultOrgAuditLogListOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		stubOrgAuditLogListFactory(c, "acme"), args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"events": [],
		"count": 0
	}`, buf.String())
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
		orgAuditLogListArgs{outputFormat: defaultOrgAuditLogListOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "listing audit logs: boom", err.Error())
	assert.Equal(t, "", buf.String())
}

func TestOrgAuditLogList_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runOrgAuditLogList(t.Context(), &buf,
		failingOrgAuditLogListFactory(errors.New("not logged in")),
		orgAuditLogListArgs{outputFormat: defaultOrgAuditLogListOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
	assert.Equal(t, "", buf.String())
}
