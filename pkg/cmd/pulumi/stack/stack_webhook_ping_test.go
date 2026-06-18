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

type mockWebhookPingClient struct {
	delivery apitype.WebhookDelivery
	err      error
	gotName  string
}

func (m *mockWebhookPingClient) PingStackWebhook(
	_ context.Context, _ client.StackIdentifier, name string,
) (apitype.WebhookDelivery, error) {
	m.gotName = name
	return m.delivery, m.err
}

func stubPingFactory(c stackWebhookPingClient) stackWebhookPingClientFactory {
	return func(_ context.Context, _ string) (stackWebhookPingClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func sampleDelivery() apitype.WebhookDelivery {
	return apitype.WebhookDelivery{
		ID:              "delivery-abc",
		Kind:            "ping",
		Payload:         `{"timestamp":1715558400,"message":"ping"}`,
		Timestamp:       1715558400,
		Duration:        42,
		RequestURL:      "https://example.com/webhook",
		RequestHeaders:  "Content-Type: application/json",
		ResponseCode:    200,
		ResponseHeaders: "Content-Type: text/plain",
		ResponseBody:    "ok",
	}
}

func TestStackWebhookPing_TextOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookPingClient{delivery: sampleDelivery()}
	err := runStackWebhookPing(t.Context(), &buf, stubPingFactory(c), "", "deploy-hook", renderWebhookPingText)
	require.NoError(t, err)

	assert.Equal(t, `ID:                delivery-abc
Kind:              ping
URL:               https://example.com/webhook
Timestamp:         2024-05-13T00:00:00Z
Duration:          42ms
Request headers:
  Content-Type: application/json
Payload:           {"timestamp":1715558400,"message":"ping"}
Response code:     200
Response body:
  ok
`, buf.String())
}

func TestStackWebhookPing_TextOutput_NoBody(t *testing.T) {
	t.Parallel()

	d := sampleDelivery()
	d.ResponseBody = ""

	var buf bytes.Buffer
	c := &mockWebhookPingClient{delivery: d}
	err := runStackWebhookPing(t.Context(), &buf, stubPingFactory(c), "", "hook", renderWebhookPingText)
	require.NoError(t, err)

	assert.Equal(t, `ID:                delivery-abc
Kind:              ping
URL:               https://example.com/webhook
Timestamp:         2024-05-13T00:00:00Z
Duration:          42ms
Request headers:
  Content-Type: application/json
Payload:           {"timestamp":1715558400,"message":"ping"}
Response code:     200
`, buf.String())
}

func TestStackWebhookPing_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookPingClient{delivery: sampleDelivery()}
	err := runStackWebhookPing(t.Context(), &buf, stubPingFactory(c), "", "deploy-hook", renderWebhookPingJSON)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"id": "delivery-abc",
		"kind": "ping",
		"payload": "{\"timestamp\":1715558400,\"message\":\"ping\"}",
		"timestamp": 1715558400,
		"duration": 42,
		"requestUrl": "https://example.com/webhook",
		"requestHeaders": "Content-Type: application/json",
		"responseCode": 200,
		"responseHeaders": "Content-Type: text/plain",
		"responseBody": "ok"
	}`, buf.String())
}

func TestStackWebhookPing_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockWebhookPingClient{err: errors.New("connection refused")}
	err := runStackWebhookPing(t.Context(), &buf, stubPingFactory(c), "", "hook", renderWebhookPingText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pinging stack webhook")
	assert.Contains(t, err.Error(), "connection refused")
}
