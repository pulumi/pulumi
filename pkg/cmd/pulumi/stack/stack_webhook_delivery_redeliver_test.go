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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRedeliverClient struct {
	delivery   apitype.WebhookDelivery
	err        error
	gotWebhook string
	gotEventID string
}

func (m *mockRedeliverClient) RedeliverStackWebhookEvent(
	_ context.Context, _ client.StackIdentifier, webhook, eventID string,
) (apitype.WebhookDelivery, error) {
	m.gotWebhook = webhook
	m.gotEventID = eventID
	return m.delivery, m.err
}

func stubRedeliverFactory(
	c stackWebhookRedeliverClient,
) stackWebhookRedeliverClientFactory {
	return func(_ context.Context, _ string) (stackWebhookRedeliverClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func redeliverSample() apitype.WebhookDelivery {
	return apitype.WebhookDelivery{
		ID:              "d-new",
		Kind:            "stack_update",
		Payload:         `{"action":"update"}`,
		Timestamp:       1715558500,
		Duration:        55,
		RequestURL:      "https://example.com/webhook",
		RequestHeaders:  "Content-Type: application/json",
		ResponseCode:    200,
		ResponseHeaders: "Content-Type: text/plain",
		ResponseBody:    "ok",
	}
}

func TestRedeliver_TextOutput(t *testing.T) {
	t.Parallel()

	c := &mockRedeliverClient{delivery: redeliverSample()}
	var buf bytes.Buffer
	err := runRedeliver(t.Context(), &buf, stubRedeliverFactory(c), "", "my-hook", "evt-1", "default")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID:                d-new")
	assert.Contains(t, out, "Kind:              stack_update")
	assert.Contains(t, out, "URL:               https://example.com/webhook")
	assert.Contains(t, out, "Duration:          55ms")
	assert.Contains(t, out, "Request headers:\n  Content-Type: application/json\n")
	assert.Contains(t, out, "Payload:           {\"action\":\"update\"}")
	assert.Contains(t, out, "Response code:     200")
	assert.Contains(t, out, "Response body:\n  ok\n")
}

func TestRedeliver_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockRedeliverClient{delivery: redeliverSample()}
	var buf bytes.Buffer
	err := runRedeliver(t.Context(), &buf, stubRedeliverFactory(c), "", "hook", "evt-1", "json")
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"id": "d-new",
		"kind": "stack_update",
		"payload": "{\"action\":\"update\"}",
		"timestamp": 1715558500,
		"duration": 55,
		"requestUrl": "https://example.com/webhook",
		"requestHeaders": "Content-Type: application/json",
		"responseCode": 200,
		"responseHeaders": "Content-Type: text/plain",
		"responseBody": "ok"
	}`, buf.String())
}
