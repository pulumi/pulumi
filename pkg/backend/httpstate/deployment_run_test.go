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

package httpstate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cancelEngineEvent constructs a minimal cancel engine event for tests.
func cancelEngineEvent(seq int) apitype.EngineEvent {
	return apitype.EngineEvent{
		Sequence:    seq,
		Timestamp:   0,
		CancelEvent: &apitype.CancelEvent{},
	}
}

// mustJSON marshals v to JSON bytes, panicking on error (test helper only).
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// TestShowDeploymentEvents_RaceConditionRetries verifies that showDeploymentEvents retries
// GetUpdateEngineEvents when the first call returns empty+nil (the NoState race condition)
// while GetDeploymentLogs reports the deployment is still running (NextToken != "").
func TestShowDeploymentEvents_RaceConditionRetries(t *testing.T) {
	t.Parallel()

	const (
		org       = "myorg"
		project   = "myproject"
		stack     = "mystack"
		deployID  = "deploy-1"
		updateID  = "update-1"
		logsToken = "log-cursor-after-header" //nolint:gosec // not a credential, just a pagination cursor
	)

	var eventsCallCount atomic.Int32
	var logsCallCount atomic.Int32

	transport := &mockTransport{
		roundTrip: func(r *http.Request) (*http.Response, error) {
			writeJSON := func(v any) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(string(mustJSON(v)))),
					Header:     make(http.Header),
				}, nil
			}

			switch {
			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/deployments/%s/updates", deployID)):
				return writeJSON([]apitype.GetDeploymentUpdatesUpdateInfo{
					{UpdateID: updateID, Version: 1},
				})

			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/update/%s/events", updateID)):
				n := eventsCallCount.Add(1)
				if n == 1 {
					// Simulate the NoState race: no events yet, nil continuation token.
					return writeJSON(apitype.GetUpdateEventsResponse{})
				}
				// Second call: executor has written events and cleared NoState.
				return writeJSON(apitype.GetUpdateEventsResponse{
					Events: []apitype.EngineEvent{cancelEngineEvent(1)},
				})

			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/deployments/%s/logs", deployID)):
				n := logsCallCount.Add(1)
				nextToken := ""
				if n == 1 {
					// First check: deployment still running.
					nextToken = "log-cursor-after-executor-step" //nolint:gosec // not a credential, just a pagination cursor
				}
				return writeJSON(apitype.DeploymentLogs{NextToken: nextToken})

			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader(`{"code":404,"message":"not found"}`)),
					Header:     make(http.Header),
				}, nil
			}
		},
	}

	apiClient := client.NewClient(client.PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
	apiClient.WithHTTPClient(&http.Client{Transport: transport})
	b := &cloudBackend{
		client: apiClient,
		d:      diagtest.LogSink(t),
	}

	stackID := client.StackIdentifier{Owner: org, Project: project, Stack: tokens.MustParseStackName(stack)}
	opts := display.Options{Color: colors.Never, SuppressPermalink: true}

	err := b.showDeploymentEvents(t.Context(), stackID, apitype.PreviewUpdate, deployID, opts, logsToken)
	require.NoError(t, err)

	assert.Equal(t, int32(2), eventsCallCount.Load(),
		"should retry GetUpdateEngineEvents when first response is empty+nil and deployment is running")
	assert.GreaterOrEqual(t, logsCallCount.Load(), int32(1),
		"should check deployment liveness via GetDeploymentLogs")
}

// TestShowDeploymentEvents_NormalPath verifies the happy path: events are returned on the
// first call and showDeploymentEvents exits cleanly without checking deployment logs.
func TestShowDeploymentEvents_NormalPath(t *testing.T) {
	t.Parallel()

	const (
		org      = "myorg"
		project  = "myproject"
		stack    = "mystack"
		deployID = "deploy-2"
		updateID = "update-2"
	)

	var eventsCallCount atomic.Int32
	var logsCallCount atomic.Int32

	transport := &mockTransport{
		roundTrip: func(r *http.Request) (*http.Response, error) {
			writeJSON := func(v any) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(string(mustJSON(v)))),
					Header:     make(http.Header),
				}, nil
			}

			switch {
			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/deployments/%s/updates", deployID)):
				return writeJSON([]apitype.GetDeploymentUpdatesUpdateInfo{
					{UpdateID: updateID, Version: 1},
				})

			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/update/%s/events", updateID)):
				eventsCallCount.Add(1)
				// Events available immediately — no race condition.
				return writeJSON(apitype.GetUpdateEventsResponse{
					Events: []apitype.EngineEvent{cancelEngineEvent(1)},
				})

			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/deployments/%s/logs", deployID)):
				logsCallCount.Add(1)
				return writeJSON(apitype.DeploymentLogs{NextToken: ""})

			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader(`{"code":404,"message":"not found"}`)),
					Header:     make(http.Header),
				}, nil
			}
		},
	}

	apiClient := client.NewClient(client.PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
	apiClient.WithHTTPClient(&http.Client{Transport: transport})
	b := &cloudBackend{
		client: apiClient,
		d:      diagtest.LogSink(t),
	}

	stackID := client.StackIdentifier{Owner: org, Project: project, Stack: tokens.MustParseStackName(stack)}
	opts := display.Options{Color: colors.Never, SuppressPermalink: true}

	err := b.showDeploymentEvents(t.Context(), stackID, apitype.PreviewUpdate, deployID, opts, "log-token")
	require.NoError(t, err)

	assert.Equal(t, int32(1), eventsCallCount.Load(),
		"should call GetUpdateEngineEvents exactly once when events are returned immediately")
	assert.Equal(t, int32(0), logsCallCount.Load(),
		"should NOT check deployment logs when events are returned on the first call")
}

// TestShowDeploymentEvents_NoEventsDeploymentDone verifies that showDeploymentEvents exits
// gracefully (without hanging) when empty+nil is returned AND the deployment is confirmed
// done (GetDeploymentLogs returns NextToken == "").
func TestShowDeploymentEvents_NoEventsDeploymentDone(t *testing.T) {
	t.Parallel()

	const (
		org      = "myorg"
		project  = "myproject"
		stack    = "mystack"
		deployID = "deploy-3"
		updateID = "update-3"
	)

	transport := &mockTransport{
		roundTrip: func(r *http.Request) (*http.Response, error) {
			writeJSON := func(v any) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(string(mustJSON(v)))),
					Header:     make(http.Header),
				}, nil
			}

			switch {
			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/deployments/%s/updates", deployID)):
				return writeJSON([]apitype.GetDeploymentUpdatesUpdateInfo{
					{UpdateID: updateID, Version: 1},
				})

			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/update/%s/events", updateID)):
				// Deployment has no engine events (e.g., failed before engine started).
				return writeJSON(apitype.GetUpdateEventsResponse{})

			case strings.HasSuffix(r.URL.Path, fmt.Sprintf("/deployments/%s/logs", deployID)):
				// Deployment is done — no more logs.
				return writeJSON(apitype.DeploymentLogs{NextToken: ""})

			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader(`{"code":404,"message":"not found"}`)),
					Header:     make(http.Header),
				}, nil
			}
		},
	}

	apiClient := client.NewClient(client.PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
	apiClient.WithHTTPClient(&http.Client{Transport: transport})
	b := &cloudBackend{
		client: apiClient,
		d:      diagtest.LogSink(t),
	}

	stackID := client.StackIdentifier{Owner: org, Project: project, Stack: tokens.MustParseStackName(stack)}
	opts := display.Options{Color: colors.Never, SuppressPermalink: true}

	// Must return without hanging even with no engine events.
	err := b.showDeploymentEvents(t.Context(), stackID, apitype.PreviewUpdate, deployID, opts, "log-token")
	require.NoError(t, err)
}
