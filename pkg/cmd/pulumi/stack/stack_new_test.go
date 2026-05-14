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

// AI Generated - needs human review

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

type capturedStackNewCall struct {
	stackID client.StackIdentifier
	tags    map[apitype.StackTagName]string
	teams   []string
	state   *apitype.UntypedDeployment
	config  *apitype.StackConfig
}

type mockStackNewClient struct {
	created  apitype.Stack
	details  client.CreateStackDetails
	err      error
	captured *capturedStackNewCall
}

func (m *mockStackNewClient) CreateStack(
	_ context.Context,
	stackID client.StackIdentifier,
	tags map[apitype.StackTagName]string,
	teams []string,
	state *apitype.UntypedDeployment,
	config *apitype.StackConfig,
) (apitype.Stack, client.CreateStackDetails, error) {
	if m.captured != nil {
		*m.captured = capturedStackNewCall{
			stackID: stackID,
			tags:    tags,
			teams:   teams,
			state:   state,
			config:  config,
		}
	}
	if m.err != nil {
		return apitype.Stack{}, client.CreateStackDetails{}, m.err
	}
	return m.created, m.details, nil
}

func stubStackNewFactory(c stackNewClient, org string) stackNewClientFactory {
	return func(_ context.Context, _ string) (stackNewClient, string, error) {
		return c, org, nil
	}
}

func failingStackNewFactory(err error) stackNewClientFactory {
	return func(_ context.Context, _ string) (stackNewClient, string, error) {
		return nil, "", err
	}
}

func TestStackNew_TextOutput(t *testing.T) {
	t.Parallel()

	c := &mockStackNewClient{
		details: client.CreateStackDetails{
			Messages: []apitype.Message{
				{Severity: apitype.MessageSeverity("info"), Message: "Welcome to your new stack."},
			},
		},
	}

	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, stubStackNewFactory(c, "acme"),
		"my-project", "dev",
		stackNewArgs{outputFormat: defaultStackNewOutputFormat()})
	require.NoError(t, err)

	expected := "Created stack acme/my-project/dev\n" +
		"Organization:   acme\n" +
		"Project:        my-project\n" +
		"Stack:          dev\n" +
		"Welcome to your new stack.\n"
	assert.Equal(t, expected, buf.String())
}

func TestStackNew_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockStackNewClient{
		details: client.CreateStackDetails{
			Messages: []apitype.Message{
				{Severity: apitype.MessageSeverity("info"), Message: "Hi."},
			},
		},
	}

	args := stackNewArgs{outputFormat: defaultStackNewOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, stubStackNewFactory(c, "acme"),
		"my-project", "dev", args)
	require.NoError(t, err)

	expected := `{
  "organizationName": "acme",
  "projectName": "my-project",
  "stackName": "dev",
  "messages": [
    {"severity": "info", "message": "Hi."}
  ]
}`
	assert.JSONEq(t, expected, buf.String())
}

func TestStackNew_JSONNormalizesNilMessages(t *testing.T) {
	t.Parallel()

	c := &mockStackNewClient{details: client.CreateStackDetails{Messages: nil}}

	args := stackNewArgs{outputFormat: defaultStackNewOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, stubStackNewFactory(c, "acme"),
		"my-project", "dev", args)
	require.NoError(t, err)

	expected := `{
  "organizationName": "acme",
  "projectName": "my-project",
  "stackName": "dev",
  "messages": []
}`
	assert.JSONEq(t, expected, buf.String())
}

func TestStackNew_FactoryError(t *testing.T) {
	t.Parallel()

	factoryErr := errors.New("user not a member of acme")
	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, failingStackNewFactory(factoryErr),
		"my-project", "dev",
		stackNewArgs{outputFormat: defaultStackNewOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, factoryErr, err)
}

func TestStackNew_ClientErrorWrapped(t *testing.T) {
	t.Parallel()

	c := &mockStackNewClient{err: errors.New("boom")}
	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, stubStackNewFactory(c, "acme"),
		"my-project", "dev",
		stackNewArgs{outputFormat: defaultStackNewOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "creating stack: boom", err.Error())
}

func TestStackNew_NoConfigFlagsSendsNilConfig(t *testing.T) {
	t.Parallel()

	var captured capturedStackNewCall
	c := &mockStackNewClient{captured: &captured}

	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, stubStackNewFactory(c, "acme"),
		"my-project", "dev",
		stackNewArgs{outputFormat: defaultStackNewOutputFormat()})
	require.NoError(t, err)
	assert.Nil(t, captured.config)
	assert.Nil(t, captured.tags)
	assert.Nil(t, captured.teams)
	assert.Nil(t, captured.state)
}

func TestStackNew_ConfigFlagsBuildStackConfig(t *testing.T) {
	t.Parallel()

	var captured capturedStackNewCall
	c := &mockStackNewClient{captured: &captured}

	var buf bytes.Buffer
	err := runStackNew(t.Context(), &buf, stubStackNewFactory(c, "acme"),
		"my-project", "dev", stackNewArgs{
			environment:     "acme/prod",
			secretsProvider: "awskms://key",
			outputFormat:    defaultStackNewOutputFormat(),
		})
	require.NoError(t, err)
	assert.Equal(t, &apitype.StackConfig{
		Environment:     "acme/prod",
		SecretsProvider: "awskms://key",
	}, captured.config)
}
