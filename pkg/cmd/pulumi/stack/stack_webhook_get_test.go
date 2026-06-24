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

// mockWebhookGetClient implements stackWebhookGetClient for tests.
type mockWebhookGetClient struct {
	webhook apitype.Webhook
	err     error
	gotName string
}

func (m *mockWebhookGetClient) GetStackWebhook(
	_ context.Context, _ client.StackIdentifier, name string,
) (apitype.Webhook, error) {
	m.gotName = name
	return m.webhook, m.err
}

func sampleWebhook() apitype.Webhook {
	return apitype.Webhook{
		OrganizationName: "my-org",
		ProjectName:      ptr("my-project"),
		StackName:        ptr("dev"),
		Name:             "deploy-hook",
		DisplayName:      "Deploy Hook",
		PayloadURL:       "https://example.com/webhook",
		Active:           true,
		Format:           ptr("raw"),
		Groups:           []string{"stacks"},
		Filters:          []string{"stack_update"},
		HasSecret:        true,
		SecretCiphertext: "v1:abc123",
	}
}

func newTestGetCmd(c *mockWebhookGetClient, render webhookGetRenderFunc) *stackWebhookGetCmd {
	return &stackWebhookGetCmd{
		client:  c,
		stackID: testStackID,
		render:  render,
	}
}

func TestStackWebhookGet_TextOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookGetClient{webhook: sampleWebhook()}
	err := newTestGetCmd(c, renderWebhookGetText).run(t.Context(), &buf, "deploy-hook")
	require.NoError(t, err)

	assert.Equal(t, `ID:                deploy-hook
Name:              Deploy Hook
Organization:      my-org
Project:           my-project
Stack:             dev
URL:               https://example.com/webhook
Format:            raw
Event groups:      stacks
Events:            stack_update
Active:            yes
Has secret:        yes
Secret ciphertext: v1:abc123
`, buf.String())
}

func TestStackWebhookGet_TextOutput_Minimal(t *testing.T) {
	t.Parallel()

	// Webhook with no display name, no format, no groups, no events.
	wh := apitype.Webhook{
		OrganizationName: "my-org",
		Name:             "bare-hook",
		PayloadURL:       "https://example.com",
		Active:           false,
	}

	var buf bytes.Buffer
	c := &mockWebhookGetClient{webhook: wh}
	err := newTestGetCmd(c, renderWebhookGetText).run(t.Context(), &buf, "bare-hook")
	require.NoError(t, err)

	assert.Equal(t, `ID:                bare-hook
Organization:      my-org
URL:               https://example.com
Active:            no
Has secret:        no
`, buf.String())
}

func TestStackWebhookGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookGetClient{webhook: sampleWebhook()}
	err := newTestGetCmd(c, renderWebhookGetJSON).run(t.Context(), &buf, "deploy-hook")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organizationName": "my-org",
		"projectName": "my-project",
		"stackName": "dev",
		"envName": "",
		"id": "deploy-hook",
		"name": "Deploy Hook",
		"url": "https://example.com/webhook",
		"format": "raw",
		"active": true,
		"eventGroups": ["stacks"],
		"events": ["stack_update"],
		"hasSecret": true,
		"secretCiphertext": "v1:abc123"
	}`, buf.String())
}

func TestStackWebhookGet_JSONOutput_NilFields(t *testing.T) {
	t.Parallel()

	wh := apitype.Webhook{
		OrganizationName: "my-org",
		Name:             "bare-hook",
		PayloadURL:       "https://example.com",
		Active:           true,
	}

	var buf bytes.Buffer
	c := &mockWebhookGetClient{webhook: wh}
	err := newTestGetCmd(c, renderWebhookGetJSON).run(t.Context(), &buf, "bare-hook")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"organizationName": "my-org",
		"projectName": "",
		"stackName": "",
		"envName": "",
		"id": "bare-hook",
		"name": "",
		"url": "https://example.com",
		"format": "",
		"active": true,
		"eventGroups": [],
		"events": [],
		"hasSecret": false,
		"secretCiphertext": ""
	}`, buf.String())
}

func TestStackWebhookGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookGetClient{err: errors.New("not found")}
	err := newTestGetCmd(c, renderWebhookGetText).run(t.Context(), &buf, "no-such")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading stack webhook")
	assert.Contains(t, err.Error(), "not found")
}

func TestStackWebhookGet_WebhookNamePropagation(t *testing.T) {
	t.Parallel()

	c := &mockWebhookGetClient{webhook: sampleWebhook()}
	var buf bytes.Buffer
	err := newTestGetCmd(c, renderWebhookGetText).run(t.Context(), &buf, "my-hook-name")
	require.NoError(t, err)
	assert.Equal(t, "my-hook-name", c.gotName)
}
