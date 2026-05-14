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

package deployment

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

// mockDeploymentGetClient stubs deploymentGetClient. It returns a fixed
// response (or error) and records the deployment ID it was called with.
type mockDeploymentGetClient struct {
	resp  apitype.GetDeploymentResponse
	err   error
	gotID string
}

func (m *mockDeploymentGetClient) GetDeployment(
	_ context.Context, _ client.StackIdentifier, id string,
) (apitype.GetDeploymentResponse, error) {
	m.gotID = id
	if m.err != nil {
		return apitype.GetDeploymentResponse{}, m.err
	}
	return m.resp, nil
}

func stubGetFactory(c deploymentGetClient) deploymentGetClientFactory {
	return func(_ context.Context, _ string) (deploymentGetClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingGetFactory(err error) deploymentGetClientFactory {
	return func(_ context.Context, _ string) (deploymentGetClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func deploymentGetJSONArgs(t *testing.T) deploymentGetArgs {
	t.Helper()
	args := deploymentGetArgs{outputFormat: defaultDeploymentGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	return args
}

func sampleGetResponse() apitype.GetDeploymentResponse {
	return apitype.GetDeploymentResponse{
		ID:              "dep-1",
		Created:         "2026-05-01T12:00:00Z",
		Modified:        "2026-05-01T12:05:00Z",
		Status:          "succeeded",
		Version:         42,
		RequestedBy:     apitype.UserInfo{Name: "Alice", GitHubLogin: "alice", AvatarURL: "https://x/a.png"},
		ProjectName:     "web",
		StackName:       "prod",
		PulumiOperation: apitype.Update,
		Initiator:       "cli",
		Updates:         []apitype.DeploymentNestedUpdate{},
		Jobs: []apitype.DeploymentJob{{
			Status: "succeeded",
			Steps: []apitype.DeploymentStepRun{
				{Name: "pulumi-up", Status: "succeeded"},
			},
		}},
		InheritSettings: true,
	}
}

func TestDeploymentGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentGetClient{resp: sampleGetResponse()}
	err := runDeploymentGet(t.Context(), &buf, stubGetFactory(c), "dep-1",
		deploymentGetArgs{outputFormat: defaultDeploymentGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `ID:                dep-1
Status:            succeeded
Operation:         update
Version:           42
Project:           web
Stack:             prod
Created:           2026-05-01T12:00:00Z
Modified:          2026-05-01T12:05:00Z
Initiated by:      alice
Initiator:         cli
Jobs:              1
Updates:           0
`, buf.String())
}

func TestDeploymentGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentGetClient{resp: sampleGetResponse()}
	err := runDeploymentGet(t.Context(), &buf, stubGetFactory(c), "dep-1",
		deploymentGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"id": "dep-1",
		"created": "2026-05-01T12:00:00Z",
		"modified": "2026-05-01T12:05:00Z",
		"status": "succeeded",
		"version": 42,
		"requestedBy": {
			"name": "Alice",
			"githubLogin": "alice",
			"avatarUrl": "https://x/a.png"
		},
		"projectName": "web",
		"stackName": "prod",
		"paused": false,
		"pulumiOperation": "update",
		"updates": [],
		"jobs": [{
			"status": "succeeded",
			"started": "0001-01-01T00:00:00Z",
			"lastUpdated": "0001-01-01T00:00:00Z",
			"steps": [{
				"name": "pulumi-up",
				"status": "succeeded",
				"started": "0001-01-01T00:00:00Z",
				"lastUpdated": "0001-01-01T00:00:00Z"
			}]
		}],
		"initiator": "cli",
		"inheritSettings": true
	}`, buf.String())
}

func TestDeploymentGet_JSONOutput_NilSlicesNormalized(t *testing.T) {
	t.Parallel()

	resp := apitype.GetDeploymentResponse{
		ID:              "dep-bare",
		Status:          "not-started",
		Version:         1,
		PulumiOperation: apitype.Preview,
		RequestedBy:     apitype.UserInfo{Name: "Bob"},
	}

	var buf bytes.Buffer
	c := &mockDeploymentGetClient{resp: resp}
	err := runDeploymentGet(t.Context(), &buf, stubGetFactory(c), "dep-bare",
		deploymentGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"id": "dep-bare",
		"created": "",
		"modified": "",
		"status": "not-started",
		"version": 1,
		"requestedBy": {
			"name": "Bob",
			"githubLogin": "",
			"avatarUrl": ""
		},
		"projectName": "",
		"stackName": "",
		"paused": false,
		"pulumiOperation": "preview",
		"updates": [],
		"jobs": [],
		"initiator": "",
		"inheritSettings": false
	}`, buf.String())
}

func TestDeploymentGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentGetClient{err: errors.New("not found")}
	err := runDeploymentGet(t.Context(), &buf, stubGetFactory(c), "dep-missing",
		deploymentGetArgs{outputFormat: defaultDeploymentGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment")
	assert.Contains(t, err.Error(), "not found")
}

func TestDeploymentGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentGet(t.Context(), &buf,
		failingGetFactory(errors.New("not logged in")), "dep-1",
		deploymentGetArgs{outputFormat: defaultDeploymentGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestDeploymentGet_DeploymentIDPropagation(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentGetClient{resp: sampleGetResponse()}
	err := runDeploymentGet(t.Context(), &buf, stubGetFactory(c), "my-dep-id",
		deploymentGetArgs{outputFormat: defaultDeploymentGetOutputFormat()})
	require.NoError(t, err)
	assert.Equal(t, "my-dep-id", c.gotID)
}
