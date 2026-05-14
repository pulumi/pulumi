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

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrgWebhookListClient struct {
	webhooks []apitype.Webhook
	err      error
	gotOrg   string
}

func (m *mockOrgWebhookListClient) ListOrgWebhooks(
	_ context.Context, org string,
) ([]apitype.Webhook, error) {
	m.gotOrg = org
	return m.webhooks, m.err
}

func ptr(s string) *string { return &s }

func sampleOrgWebhooks() []apitype.Webhook {
	return []apitype.Webhook{
		{
			OrganizationName: "my-org",
			Name:             "deploy-hook",
			DisplayName:      "Deploy Hook",
			PayloadURL:       "https://example.com/webhook",
			Active:           true,
			Format:           ptr("raw"),
			Groups:           []string{"stacks", "deployments"},
			Filters:          []string{"stack_created"},
		},
		{
			OrganizationName: "my-org",
			Name:             "slack-hook",
			DisplayName:      "Slack Alerts",
			PayloadURL:       "https://hooks.slack.com/T00",
			Active:           false,
			Format:           ptr("slack"),
		},
	}
}

func newTestOrgWebhookListCmd(
	client orgWebhookListClient, orgName string,
) (*orgWebhookListCmd, *bytes.Buffer) {
	var buf bytes.Buffer
	return &orgWebhookListCmd{
		orgName: orgName,
		output: outputflag.OutputFlag[orgWebhookListRender]{
			RenderForTerminal: (*orgWebhookListCmd).renderTable,
			RenderJSON:        (*orgWebhookListCmd).renderJSON,
		},
		w: &buf,
	}, &buf
}

func TestOrgWebhookList_TableOutput(t *testing.T) {
	t.Parallel()

	mc := &mockOrgWebhookListClient{webhooks: sampleOrgWebhooks()}
	olcmd, buf := newTestOrgWebhookListCmd(mc, "my-org")
	err := olcmd.output.Get()(olcmd, mc.webhooks)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "URL")
	assert.Contains(t, out, "ACTIVE")
	assert.Contains(t, out, "deploy-hook")
	assert.Contains(t, out, "Deploy Hook")
	assert.Contains(t, out, "slack-hook")
	assert.Contains(t, out, "Slack Alerts")
	assert.Contains(t, out, "stacks")
	assert.Contains(t, out, "deployments")
	assert.Contains(t, out, "stack_created")
	assert.Contains(t, out, "yes")
	assert.Contains(t, out, "no")
	assert.Contains(t, out, "2 webhook(s)")
}

func TestOrgWebhookList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	mc := &mockOrgWebhookListClient{webhooks: []apitype.Webhook{}}
	olcmd, buf := newTestOrgWebhookListCmd(mc, "my-org")
	err := olcmd.output.Get()(olcmd, mc.webhooks)
	require.NoError(t, err)

	assert.Equal(t, "No webhooks configured for this organization.\n", buf.String())
}

func TestOrgWebhookList_JSONOutput(t *testing.T) {
	t.Parallel()

	mc := &mockOrgWebhookListClient{webhooks: sampleOrgWebhooks()}
	olcmd, buf := newTestOrgWebhookListCmd(mc, "my-org")
	require.NoError(t, olcmd.output.Set("json"))
	err := olcmd.output.Get()(olcmd, mc.webhooks)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, `"id": "deploy-hook"`)
	assert.Contains(t, out, `"name": "Deploy Hook"`)
	assert.Contains(t, out, `"count": 2`)
	assert.Contains(t, out, `"eventGroups"`)
	assert.Contains(t, out, `"events"`)
}

func TestOrgWebhookList_DropsEmptyColumns(t *testing.T) {
	t.Parallel()

	webhooks := []apitype.Webhook{
		{
			OrganizationName: "my-org",
			Name:             "hook-a",
			PayloadURL:       "https://example.com",
			Active:           true,
		},
	}
	mc := &mockOrgWebhookListClient{webhooks: webhooks}
	olcmd, buf := newTestOrgWebhookListCmd(mc, "my-org")
	err := olcmd.output.Get()(olcmd, mc.webhooks)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "URL")
	assert.Contains(t, out, "ACTIVE")
	assert.NotContains(t, out, "NAME")
	assert.NotContains(t, out, "FORMAT")
	assert.NotContains(t, out, "EVENT GROUPS")
	assert.NotContains(t, out, "EVENTS")
}
