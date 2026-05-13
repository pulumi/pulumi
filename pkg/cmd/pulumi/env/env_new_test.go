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

package env

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedNewCall struct {
	org, project, name string
	yaml               []byte
}

type mockEnvNewClient struct {
	captured *capturedNewCall
	err      error
}

func (m *mockEnvNewClient) CreateEnvironment(
	_ context.Context, org, project, name string, yaml []byte,
) (any, error) {
	if m.captured != nil {
		*m.captured = capturedNewCall{org: org, project: project, name: name, yaml: yaml}
	}
	return nil, m.err
}

func stubNewFactory(c envNewClient, defaultOrg string) envNewFactory {
	return func(_ context.Context, orgOverride string) (envNewClient, string, error) {
		org := orgOverride
		if org == "" {
			org = defaultOrg
		}
		return c, org, nil
	}
}

func TestEnvNew_Success(t *testing.T) {
	t.Parallel()

	var captured capturedNewCall
	c := &mockEnvNewClient{captured: &captured}

	var out bytes.Buffer
	err := runEnvNew(t.Context(), &out, stubNewFactory(c, "acme"), "", "my-project", "my-env")
	require.NoError(t, err)

	assert.Equal(t, capturedNewCall{org: "acme", project: "my-project", name: "my-env"}, captured)
	assert.Contains(t, out.String(), "Created environment acme/my-project/my-env")
}

func TestEnvNew_OrgOverride(t *testing.T) {
	t.Parallel()

	var captured capturedNewCall
	c := &mockEnvNewClient{captured: &captured}

	var out bytes.Buffer
	err := runEnvNew(t.Context(), &out, stubNewFactory(c, "acme"), "other-co", "my-project", "my-env")
	require.NoError(t, err)
	assert.Equal(t, "other-co", captured.org)
}

func TestEnvNew_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockEnvNewClient{err: errors.New("409 conflict")}

	var out bytes.Buffer
	err := runEnvNew(t.Context(), &out, stubNewFactory(c, "acme"), "", "my-project", "my-env")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating environment my-project/my-env in org acme")
	assert.Contains(t, err.Error(), "409 conflict")
}

func TestEnvNew_FactoryError(t *testing.T) {
	t.Parallel()

	factory := func(_ context.Context, _ string) (envNewClient, string, error) {
		return nil, "", errors.New("not logged in")
	}
	var out bytes.Buffer
	err := runEnvNew(t.Context(), &out, factory, "", "p", "n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestNewEnvNewCmd_FlagBinding(t *testing.T) {
	t.Parallel()

	var captured capturedNewCall
	c := &mockEnvNewClient{captured: &captured}
	cmd := newEnvNewCmdWith(stubNewFactory(c, "acme"))
	assert.Equal(t, "new", cmd.Name())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--org", "other-co", "my-project", "my-env"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	assert.Equal(t, capturedNewCall{org: "other-co", project: "my-project", name: "my-env"}, captured)
}
