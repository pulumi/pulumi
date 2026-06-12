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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEditClient struct {
	existing  apitype.Webhook
	getErr    error
	updated   apitype.Webhook
	updateErr error
	gotReq    apitype.Webhook
	gotName   string
}

func (m *mockEditClient) GetStackWebhook(
	_ context.Context, _ client.StackIdentifier, name string,
) (apitype.Webhook, error) {
	return m.existing, m.getErr
}

func (m *mockEditClient) UpdateStackWebhook(
	_ context.Context, _ client.StackIdentifier, name string, req apitype.Webhook,
) (apitype.Webhook, error) {
	m.gotName = name
	m.gotReq = req
	return m.updated, m.updateErr
}

func stubEditFactory(c stackWebhookEditClient) stackWebhookEditClientFactory {
	return func(_ context.Context, _ string) (stackWebhookEditClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func existingWebhook() apitype.Webhook {
	return apitype.Webhook{
		OrganizationName: "my-org",
		ProjectName:      ptr("my-project"),
		StackName:        ptr("dev"),
		Name:             "my-hook",
		DisplayName:      "My Hook",
		PayloadURL:       "https://example.com/webhook",
		Active:           true,
		Format:           ptr("raw"),
		Groups:           []string{"stacks"},
		Filters:          []string{"deployment_queued"},
		HasSecret:        true,
	}
}

func TestEdit_ChangeURL(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}
	c.updated.PayloadURL = "https://new-url.example.com"

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"my-hook", "--url", "https://new-url.example.com"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	// Only URL should have changed in the request.
	assert.Equal(t, "https://new-url.example.com", c.gotReq.PayloadURL)
	// Everything else preserved.
	assert.Equal(t, "My Hook", c.gotReq.DisplayName)
	assert.True(t, c.gotReq.Active)
	assert.Equal(t, []string{"stacks"}, c.gotReq.Groups)
	assert.Equal(t, []string{"deployment_queued"}, c.gotReq.Filters)
}

func TestEdit_ChangeActive(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}
	c.updated.Active = false

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"my-hook", "--active=false"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.False(t, c.gotReq.Active)
	// URL preserved.
	assert.Equal(t, "https://example.com/webhook", c.gotReq.PayloadURL)
}

func TestEdit_AddEvents(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--add-event", "deployment_started",
		"--add-event", "deployment_failed",
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	// Original event preserved, new ones added.
	assert.Equal(t, []string{"deployment_queued", "deployment_started", "deployment_failed"}, c.gotReq.Filters)
	// Groups unchanged.
	assert.Equal(t, []string{"stacks"}, c.gotReq.Groups)
}

func TestEdit_RemoveEvents(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	existing.Filters = []string{"deployment_queued", "deployment_started", "deployment_failed"}
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--remove-event", "deployment_started",
		"--remove-event", "deployment_failed",
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{"deployment_queued"}, c.gotReq.Filters)
}

func TestEdit_AddAndRemoveEvents(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	existing.Filters = []string{"deployment_queued", "deployment_started"}
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--remove-event", "deployment_started",
		"--add-event", "deployment_failed",
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	// deployment_started removed, deployment_failed added.
	assert.Equal(t, []string{"deployment_queued", "deployment_failed"}, c.gotReq.Filters)
}

func TestEdit_AddGroups(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	existing.Groups = []string{"stacks"}
	existing.Filters = nil
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--add-group", "deployments",
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{"stacks", "deployments"}, c.gotReq.Groups)
}

func TestEdit_RemoveGroups(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	existing.Groups = []string{"stacks", "deployments"}
	existing.Filters = nil
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--remove-group", "deployments",
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{"stacks"}, c.gotReq.Groups)
}

func TestEdit_AddDuplicateEventIsIdempotent(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--add-event", "deployment_queued", // already present
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{"deployment_queued"}, c.gotReq.Filters)
}

func TestEdit_RemoveNonexistentEventIsIdempotent(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"my-hook",
		"--remove-event", "deployment_started", // not present
	})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	// Original filter unchanged.
	assert.Equal(t, []string{"deployment_queued"}, c.gotReq.Filters)
}

func TestEdit_ClearSecret(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"my-hook", "--secret", ""})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, removeSecretSentinel, c.gotReq.Secret)
}

func TestEdit_SetSecret(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"my-hook", "--secret", "new-secret"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "new-secret", c.gotReq.Secret)
}

func TestEdit_JSONOutput(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"my-hook", "--active=false", "--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"id": "my-hook"`)
}

func TestEdit_WebhookNotFound(t *testing.T) {
	t.Parallel()

	c := &mockEditClient{getErr: errors.New("[404] Not Found")}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"bad-hook", "--active=false"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `reading webhook "bad-hook"`)
}

func TestEdit_UpdateError(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{
		existing:  existing,
		updateErr: errors.New("server error"),
	}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"my-hook", "--url", "https://x.com"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `updating webhook "my-hook"`)
}

func TestEdit_InvalidGroup(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"my-hook", "--add-group", "bogus"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid group "bogus"`)
}

func TestEdit_InvalidEvent(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"my-hook", "--add-event", "not_a_real_event"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid event "not_a_real_event"`)
}

func TestEdit_EventCoveredByGroup(t *testing.T) {
	t.Parallel()

	existing := existingWebhook()
	existing.Groups = nil
	existing.Filters = nil
	c := &mockEditClient{existing: existing, updated: existing}

	cmd := newStackWebhookEditCmdWith(stubEditFactory(c))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// update_succeeded is already in the "stacks" group
	cmd.SetArgs([]string{
		"my-hook",
		"--add-group", "stacks",
		"--add-event", "update_succeeded",
	})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `event "update_succeeded" is already included by group "stacks"`)
}

func TestEdit_DefaultCmd(t *testing.T) {
	t.Parallel()

	cmd := newStackWebhookEditCmd()
	assert.Contains(t, cmd.Use, "edit")
	require.NotNil(t, cmd.RunE)

	for _, name := range []string{
		"url", "hook-format",
		"add-event", "remove-event",
		"add-group", "remove-group",
		"active", "secret",
		"display-name", "output",
	} {
		f := cmd.Flags().Lookup(name)
		require.NotNil(t, f, "expected flag %q", name)
	}
}
