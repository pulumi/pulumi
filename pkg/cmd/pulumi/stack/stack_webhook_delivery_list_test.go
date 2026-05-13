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

type mockDeliveryListClient struct {
	deliveries []apitype.WebhookDelivery
	err        error
	getErr     error
	gotName    string
}

func (m *mockDeliveryListClient) GetStackWebhook(
	_ context.Context, _ client.StackIdentifier, _ string,
) (apitype.Webhook, error) {
	if m.getErr != nil {
		return apitype.Webhook{}, m.getErr
	}
	return apitype.Webhook{}, nil
}

func (m *mockDeliveryListClient) ListStackWebhookDeliveries(
	_ context.Context, _ client.StackIdentifier, name string,
) ([]apitype.WebhookDelivery, error) {
	m.gotName = name
	return m.deliveries, m.err
}

func stubDeliveryListFactory(
	c stackWebhookDeliveryListClient,
) stackWebhookDeliveryListClientFactory {
	return func(_ context.Context, _ string) (stackWebhookDeliveryListClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func sampleDeliveries() []apitype.WebhookDelivery {
	return []apitype.WebhookDelivery{
		{
			ID:           "d-1",
			Kind:         "stack_update",
			Timestamp:    1715558400,
			Duration:     120,
			RequestURL:   "https://example.com/webhook",
			ResponseCode: 200,
		},
		{
			ID:           "d-2",
			Kind:         "ping",
			Timestamp:    1715558300,
			Duration:     42,
			RequestURL:   "https://example.com/webhook",
			ResponseCode: 500,
			ResponseBody: "error",
		},
	}
}

func TestDeliveryList_TableOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: sampleDeliveries()}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "my-hook", "default")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "KIND")
	assert.Contains(t, out, "TIMESTAMP")
	assert.Contains(t, out, "DURATION")
	assert.Contains(t, out, "STATUS")

	assert.Contains(t, out, "d-1")
	assert.Contains(t, out, "stack_update")
	assert.Contains(t, out, "2024-05-13T00:00:00Z")
	assert.Contains(t, out, "120ms")
	assert.Contains(t, out, "200")

	assert.Contains(t, out, "d-2")
	assert.Contains(t, out, "ping")
	assert.Contains(t, out, "500")

	assert.Contains(t, out, "2 delivery(ies)")
}

func TestDeliveryList_TableOutput_Empty(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: []apitype.WebhookDelivery{}}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "hook", "table")
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No deliveries found for this webhook.")
}

func TestDeliveryList_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: sampleDeliveries()}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "my-hook", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"deliveries": [
			{
				"id": "d-1",
				"kind": "stack_update",
				"payload": "",
				"timestamp": 1715558400,
				"duration": 120,
				"requestUrl": "https://example.com/webhook",
				"requestHeaders": "",
				"responseCode": 200,
				"responseHeaders": "",
				"responseBody": ""
			},
			{
				"id": "d-2",
				"kind": "ping",
				"payload": "",
				"timestamp": 1715558300,
				"duration": 42,
				"requestUrl": "https://example.com/webhook",
				"requestHeaders": "",
				"responseCode": 500,
				"responseHeaders": "",
				"responseBody": "error"
			}
		],
		"count": 2
	}`, buf.String())
}

func TestDeliveryList_JSONOutput_Empty(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: []apitype.WebhookDelivery{}}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "hook", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{"deliveries": [], "count": 0}`, buf.String())
}

func TestDeliveryList_InvalidOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: sampleDeliveries()}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "hook", "xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --output value")
}

func TestDeliveryList_WebhookNotFound(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{getErr: errors.New("[404] Not Found")}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "typo-hook", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `webhook "typo-hook" not found`)
}

func TestDeliveryList_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{err: errors.New("not found")}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "hook", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing webhook deliveries")
}

func TestDeliveryList_WebhookNamePropagation(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: []apitype.WebhookDelivery{}}
	var buf bytes.Buffer
	err := runDeliveryList(t.Context(), &buf, stubDeliveryListFactory(c), "", "my-hook", "default")
	require.NoError(t, err)
	assert.Equal(t, "my-hook", c.gotName)
}

func TestDeliveryList_CobraFlagBinding(t *testing.T) {
	t.Parallel()

	c := &mockDeliveryListClient{deliveries: sampleDeliveries()}
	cmd := newStackWebhookDeliveryListCmdWith(stubDeliveryListFactory(c))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"my-hook", "--output", "json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"count": 2`)
}

func TestDeliveryList_DefaultCmd(t *testing.T) {
	t.Parallel()

	cmd := newStackWebhookDeliveryListCmd()
	assert.Contains(t, cmd.Use, "list")
	require.NotNil(t, cmd.RunE)

	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f)
	assert.Equal(t, "o", f.Shorthand)

	sf := cmd.Flags().Lookup("stack")
	require.NotNil(t, sf)
	assert.Equal(t, "s", sf.Shorthand)
}
