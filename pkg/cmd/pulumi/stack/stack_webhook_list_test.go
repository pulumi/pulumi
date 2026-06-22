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

package stack

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWebhookClient implements stackWebhookListClient for tests.
type mockWebhookClient struct {
	webhooks []apitype.Webhook
	err      error
}

func (m *mockWebhookClient) ListStackWebhooks(_ context.Context, _ client.StackIdentifier) ([]apitype.Webhook, error) {
	return m.webhooks, m.err
}

var testStackID = client.StackIdentifier{
	Owner:   "my-org",
	Project: "my-project",
	Stack:   tokens.MustParseStackName("dev"),
}

func stubFactory(c stackWebhookListClient) stackWebhookListClientFactory {
	return func(_ context.Context, _ string) (stackWebhookListClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingFactory(err error) stackWebhookListClientFactory {
	return func(_ context.Context, _ string) (stackWebhookListClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func ptr(s string) *string { return &s }

func sampleWebhooks() []apitype.Webhook {
	return []apitype.Webhook{
		{
			OrganizationName: "my-org",
			Name:             "deploy-hook",
			DisplayName:      "Deploy Hook",
			PayloadURL:       "https://example.com/webhook",
			Active:           true,
			Format:           ptr("raw"),
			Groups:           []string{"stacks"},
			Filters:          []string{"stack_update"},
		},
		{
			OrganizationName: "my-org",
			Name:             "slack-hook",
			DisplayName:      "Slack Notifications",
			PayloadURL:       "https://hooks.slack.com/services/T00/B00/xxx",
			Active:           false,
			Format:           ptr("slack"),
			Groups:           []string{"stacks", "deployments"},
			Filters:          []string{"stack_update", "drift_detected"},
		},
	}
}

func TestStackWebhookList_TableOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: sampleWebhooks()}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListTable)
	require.NoError(t, err)

	out := buf.String()
	// Table should contain column headers
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "URL")
	assert.Contains(t, out, "FORMAT")
	assert.Contains(t, out, "EVENT GROUPS")
	assert.Contains(t, out, "EVENTS")
	assert.Contains(t, out, "ACTIVE")

	// Table should contain data
	assert.Contains(t, out, "deploy-hook")
	assert.Contains(t, out, "Deploy Hook")
	assert.Contains(t, out, "https://example.com")
	assert.Contains(t, out, "raw")
	assert.Contains(t, out, "stacks")
	assert.Contains(t, out, "stack_update")
	assert.Contains(t, out, "yes")

	assert.Contains(t, out, "slack-hook")
	assert.Contains(t, out, "Slack Notifications")
	assert.Contains(t, out, "deployments")
	assert.Contains(t, out, "no")

	// Footer count
	assert.Contains(t, out, "2 webhook(s)")
}

func TestStackWebhookList_TableOutput_DropsEmptyColumns(t *testing.T) {
	t.Parallel()

	// Webhooks with no name, no format, no groups, and no events
	// should produce a table that omits those columns entirely.
	webhooks := []apitype.Webhook{
		{
			OrganizationName: "my-org",
			Name:             "hook-a",
			PayloadURL:       "https://example.com/a",
			Active:           true,
		},
		{
			OrganizationName: "my-org",
			Name:             "hook-b",
			PayloadURL:       "https://example.com/b",
			Active:           false,
		},
	}

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: webhooks}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListTable)
	require.NoError(t, err)

	out := buf.String()
	// Only ID, URL, and ACTIVE should be present.
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "URL")
	assert.Contains(t, out, "ACTIVE")

	// These columns should be absent since all values are empty.
	// NAME header would appear between ID and URL if present.
	assert.NotContains(t, out, "FORMAT")
	assert.NotContains(t, out, "EVENT GROUPS")
	assert.NotContains(t, out, "EVENTS")

	assert.Contains(t, out, "hook-a")
	assert.Contains(t, out, "hook-b")
	assert.Contains(t, out, "2 webhook(s)")
}

func TestStackWebhookList_TableOutput_PartialColumns(t *testing.T) {
	t.Parallel()

	// One webhook has groups, neither has events — only EVENT GROUPS column should appear.
	webhooks := []apitype.Webhook{
		{
			OrganizationName: "my-org",
			Name:             "hook-a",
			PayloadURL:       "https://example.com/a",
			Active:           true,
			Groups:           []string{"stacks"},
		},
		{
			OrganizationName: "my-org",
			Name:             "hook-b",
			PayloadURL:       "https://example.com/b",
			Active:           true,
		},
	}

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: webhooks}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListTable)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "EVENT GROUPS")
	assert.Contains(t, out, "stacks")
	assert.NotContains(t, out, "EVENTS")
	assert.NotContains(t, out, "FORMAT")
}

func TestStackWebhookList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: []apitype.Webhook{}}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListTable)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No webhooks configured for this stack.")
}

func TestStackWebhookList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: sampleWebhooks()}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListJSON)
	require.NoError(t, err)

	out := buf.String()
	assert.JSONEq(t, `{
		"webhooks": [
			{
				"id": "deploy-hook",
				"name": "Deploy Hook",
				"url": "https://example.com/webhook",
				"format": "raw",
				"active": true,
				"eventGroups": ["stacks"],
				"events": ["stack_update"]
			},
			{
				"id": "slack-hook",
				"name": "Slack Notifications",
				"url": "https://hooks.slack.com/services/T00/B00/xxx",
				"format": "slack",
				"active": false,
				"eventGroups": ["stacks", "deployments"],
				"events": ["stack_update", "drift_detected"]
			}
		],
		"count": 2
	}`, out)
}

func TestStackWebhookList_JSONOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: []apitype.Webhook{}}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{"webhooks": [], "count": 0}`, buf.String())
}

func TestStackWebhookList_JSONOutput_NilFormat(t *testing.T) {
	t.Parallel()

	// Webhook with nil Format pointer and nil Groups should render as empty values in JSON.
	webhooks := []apitype.Webhook{
		{
			OrganizationName: "my-org",
			Name:             "no-format",
			DisplayName:      "No Format",
			PayloadURL:       "https://example.com",
			Active:           true,
		},
	}

	var buf bytes.Buffer
	c := &mockWebhookClient{webhooks: webhooks}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"webhooks": [
			{
				"id": "no-format",
				"name": "No Format",
				"url": "https://example.com",
				"format": "",
				"active": true,
				"eventGroups": [],
				"events": []
			}
		],
		"count": 1
	}`, buf.String())
}

func TestStackWebhookList_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookClient{err: errors.New("server error")}
	err := runStackWebhookList(t.Context(), &buf, stubFactory(c), "", renderWebhookListTable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing stack webhooks")
	assert.Contains(t, err.Error(), "server error")
}

func TestStackWebhookList_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runStackWebhookList(t.Context(), &buf, failingFactory(errors.New("not logged in")), "", renderWebhookListTable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestStackWebhookList_StackFlagPropagation(t *testing.T) {
	t.Parallel()

	var capturedStack string
	factory := func(_ context.Context, stackFlag string) (stackWebhookListClient, client.StackIdentifier, error) {
		capturedStack = stackFlag
		return &mockWebhookClient{webhooks: []apitype.Webhook{}}, testStackID, nil
	}

	var buf bytes.Buffer
	err := runStackWebhookList(t.Context(), &buf, factory, "org/proj/my-stack", renderWebhookListTable)
	require.NoError(t, err)
	assert.Equal(t, "org/proj/my-stack", capturedStack)
}

func TestStackWebhookList_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	c := &mockWebhookClient{webhooks: sampleWebhooks()}
	cmd := newStackWebhookListCmdWith(stubFactory(c))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"count": 2`)
}

func TestStackWebhookList_DefaultCmd(t *testing.T) {
	t.Parallel()

	// Verify that newStackWebhookListCmd() returns a well-formed command.
	cmd := newStackWebhookListCmd()
	assert.Equal(t, "list", cmd.Use)
	require.NotNil(t, cmd.RunE)

	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f)
	assert.Equal(t, "", f.Shorthand)
	assert.Equal(t, "default", f.DefValue)

	sf := cmd.Flags().Lookup("stack")
	require.NotNil(t, sf)
	assert.Equal(t, "s", sf.Shorthand)
}
