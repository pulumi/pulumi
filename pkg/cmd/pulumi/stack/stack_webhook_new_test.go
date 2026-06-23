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
	"io"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWebhookNewClient struct {
	created apitype.Webhook
	err     error
	gotReq  apitype.Webhook
}

func (m *mockWebhookNewClient) CreateStackWebhook(
	_ context.Context, _ client.StackIdentifier, req apitype.Webhook,
) (apitype.Webhook, error) {
	m.gotReq = req
	return m.created, m.err
}

func stubNewFactory(c stackWebhookNewClient) stackWebhookNewClientFactory {
	return func(_ context.Context, _ string) (stackWebhookNewClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingNewFactory(err error) stackWebhookNewClientFactory {
	return func(_ context.Context, _ string) (stackWebhookNewClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func createdWebhook() apitype.Webhook {
	return apitype.Webhook{
		OrganizationName: "my-org",
		ProjectName:      ptr("my-project"),
		StackName:        ptr("dev"),
		Name:             "my-hook",
		DisplayName:      "My Hook",
		PayloadURL:       "https://example.com/webhook",
		Active:           true,
		Format:           ptr("raw"),
		HasSecret:        true,
	}
}

func TestStackWebhookNew_TextOutput(t *testing.T) {
	t.Parallel()

	c := &mockWebhookNewClient{created: createdWebhook()}
	args := stackWebhookNewArgs{
		Name:   "My Hook",
		URL:    "https://example.com/webhook",
		Format: "raw",
		Active: true,
		Secret: "s3cret",
	}

	var buf bytes.Buffer
	err := runStackWebhookNew(t.Context(), &buf, stubNewFactory(c), "", args, renderWebhookGetText)
	require.NoError(t, err)

	expected := "ID:                my-hook\n" +
		"Name:              My Hook\n" +
		"Organization:      my-org\n" +
		"Project:           my-project\n" +
		"Stack:             dev\n" +
		"URL:               https://example.com/webhook\n" +
		"Format:            raw\n" +
		"Active:            yes\n" +
		"Has secret:        yes\n"
	assert.Equal(t, expected, buf.String())
}

func TestStackWebhookNew_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockWebhookNewClient{created: createdWebhook()}
	args := stackWebhookNewArgs{
		Name:   "My Hook",
		URL:    "https://example.com/webhook",
		Format: "raw",
		Active: true,
	}

	var buf bytes.Buffer
	err := runStackWebhookNew(t.Context(), &buf, stubNewFactory(c), "", args, renderWebhookGetJSON)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), `"id": "my-hook"`)
	assert.Contains(t, buf.String(), `"organizationName": "my-org"`)
}

func TestStackWebhookNew_RequestFields(t *testing.T) {
	t.Parallel()

	c := &mockWebhookNewClient{created: createdWebhook()}
	args := stackWebhookNewArgs{
		Name:    "My Hook",
		URL:     "https://example.com/webhook",
		Format:  "slack",
		Filters: []string{"update_succeeded", "update_failed"},
		Groups:  []string{"stacks"},
		Active:  true,
		Secret:  "s3cret",
	}

	var buf bytes.Buffer
	err := runStackWebhookNew(t.Context(), &buf, stubNewFactory(c), "", args, renderWebhookGetText)
	require.NoError(t, err)

	assert.Equal(t, "", c.gotReq.Name)
	assert.Equal(t, "My Hook", c.gotReq.DisplayName)
	assert.Equal(t, "https://example.com/webhook", c.gotReq.PayloadURL)
	require.NotNil(t, c.gotReq.Format)
	assert.Equal(t, "slack", *c.gotReq.Format)
	assert.Equal(t, []string{"update_succeeded", "update_failed"}, c.gotReq.Filters)
	assert.Equal(t, []string{"stacks"}, c.gotReq.Groups)
	assert.True(t, c.gotReq.Active)
	assert.Equal(t, "s3cret", c.gotReq.Secret)
}

func TestStackWebhookNew_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockWebhookNewClient{err: errors.New("409 conflict")}
	args := stackWebhookNewArgs{
		Name: "hook", URL: "https://example.com", Format: "raw", Active: true,
	}

	var buf bytes.Buffer
	err := runStackWebhookNew(t.Context(), &buf, stubNewFactory(c), "", args, renderWebhookGetText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating stack webhook")
	assert.Contains(t, err.Error(), "409 conflict")
}

func TestStackWebhookNew_FactoryError(t *testing.T) {
	t.Parallel()

	args := stackWebhookNewArgs{
		Name: "hook", URL: "https://example.com", Format: "raw", Active: true,
	}

	var buf bytes.Buffer
	err := runStackWebhookNew(
		t.Context(), &buf, failingNewFactory(errors.New("not logged in")),
		"", args, renderWebhookGetText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestStackWebhookNew_StackFlagPropagation(t *testing.T) {
	t.Parallel()

	var capturedStack string
	factory := func(_ context.Context, stackFlag string) (stackWebhookNewClient, client.StackIdentifier, error) {
		capturedStack = stackFlag
		return &mockWebhookNewClient{created: createdWebhook()}, testStackID, nil
	}

	args := stackWebhookNewArgs{
		Name: "hook", URL: "https://example.com", Format: "raw", Active: true,
	}

	var buf bytes.Buffer
	err := runStackWebhookNew(t.Context(), &buf, factory, "org/proj/my-stack", args, renderWebhookGetText)
	require.NoError(t, err)
	assert.Equal(t, "org/proj/my-stack", capturedStack)
}

func TestStackWebhookNew_ResolveArgs_Yes(t *testing.T) {
	t.Parallel()

	// With skipPrompts=true and --name/--url set, should use values without prompting.
	args, err := resolveNewArgs(io.Discard, true, "My Hook", "https://example.com", "raw",
		nil, nil, display.Options{})
	require.NoError(t, err)

	assert.Equal(t, "My Hook", args.Name)
	assert.Equal(t, "https://example.com", args.URL)
	assert.Equal(t, "raw", args.Format)
	assert.Empty(t, args.Filters)
	assert.Empty(t, args.Groups)
}

func TestStackWebhookNew_ResolveArgs_YesNoName(t *testing.T) {
	t.Parallel()

	// With skipPrompts=true but no name, should error because name is required.
	_, err := resolveNewArgs(io.Discard, true, "", "https://example.com", "raw",
		nil, nil, display.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook name is required")
	assert.Contains(t, err.Error(), "--name")
}

func TestStackWebhookNew_ResolveArgs_YesNoURL(t *testing.T) {
	t.Parallel()

	// With skipPrompts=true but no URL, should error because URL is required.
	_, err := resolveNewArgs(io.Discard, true, "My Hook", "", "raw",
		nil, nil, display.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload URL is required")
	assert.Contains(t, err.Error(), "--url")
}

func TestFiltersNotCoveredByGroups(t *testing.T) {
	t.Parallel()

	// All groups selected → no remaining filters.
	assert.Empty(t, filtersNotCoveredByGroups([]string{"stacks", "deployments", "policies"}))

	// Only stacks → deployments + policies filters remain.
	remaining := filtersNotCoveredByGroups([]string{"stacks"})
	assert.Contains(t, remaining, "deployment_queued")
	assert.Contains(t, remaining, "policy_violation_mandatory")
	assert.NotContains(t, remaining, "update_succeeded")

	// No groups → all filters remain.
	all := filtersNotCoveredByGroups(nil)
	assert.Contains(t, all, "update_succeeded")
	assert.Contains(t, all, "deployment_queued")
	assert.Contains(t, all, "policy_violation_mandatory")
}
