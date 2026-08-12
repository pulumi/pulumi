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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWebhookRemoveClient struct {
	err     error
	gotName string
}

func (m *mockWebhookRemoveClient) DeleteStackWebhook(
	_ context.Context, _ client.StackIdentifier, name string,
) error {
	m.gotName = name
	return m.err
}

func stubRemoveFactory(c stackWebhookRemoveClient) stackWebhookRemoveClientFactory {
	return func(_ context.Context, _ string) (stackWebhookRemoveClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func TestStackWebhookRemove_Success(t *testing.T) {
	t.Parallel()

	c := &mockWebhookRemoveClient{}
	var buf bytes.Buffer
	err := runStackWebhookRemove(
		t.Context(), &buf, stubRemoveFactory(c),
		"", "my-hook", true, // yes=true to skip confirmation
	)
	require.NoError(t, err)

	assert.Equal(t, "my-hook", c.gotName)
	assert.Contains(t, buf.String(), "Webhook 'my-hook' has been removed.")
}

func TestStackWebhookRemove_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockWebhookRemoveClient{err: errors.New("not found")}
	var buf bytes.Buffer
	err := runStackWebhookRemove(
		t.Context(), &buf, stubRemoveFactory(c),
		"", "no-such", true,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing stack webhook")
	assert.Contains(t, err.Error(), "not found")
}
