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

package policy

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockPolicyGroupNewClient stubs policyGroupNewClient. It records calls and
// optionally returns fixed responses or errors for each method.
type mockPolicyGroupNewClient struct {
	getResp   apitype.GetPolicyGroupResponse
	createErr error
	getErr    error

	createdOrg  string
	createdName string
	getOrg      string
	getName     string
	createCalls int
	getCalls    int
}

func (m *mockPolicyGroupNewClient) CreatePolicyGroup(
	_ context.Context, org string, req apitype.CreatePolicyGroupRequest,
) error {
	m.createCalls++
	m.createdOrg = org
	m.createdName = req.Name
	return m.createErr
}

func (m *mockPolicyGroupNewClient) GetPolicyGroup(
	_ context.Context, org, name string,
) (apitype.GetPolicyGroupResponse, error) {
	m.getCalls++
	m.getOrg = org
	m.getName = name
	if m.getErr != nil {
		return apitype.GetPolicyGroupResponse{}, m.getErr
	}
	return m.getResp, nil
}

func stubPolicyGroupNewFactory(c policyGroupNewClient, org string) policyGroupNewClientFactory {
	return func(_ context.Context, _ string) (policyGroupNewClient, string, error) {
		return c, org, nil
	}
}

func failingPolicyGroupNewFactory(err error) policyGroupNewClientFactory {
	return func(_ context.Context, _ string) (policyGroupNewClient, string, error) {
		return nil, "", err
	}
}

func newPolicyGroupResponseFixture() apitype.GetPolicyGroupResponse {
	return apitype.GetPolicyGroupResponse{
		Name:         "prod-policies",
		IsOrgDefault: false,
		EntityType:   apitype.Stacks,
		Mode:         apitype.PolicyGroupModePreventative,
	}
}

func TestPolicyGroupNew_DefaultOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{entityType: "stacks", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  prod-policies
Org default:           no
Entity type:           stacks
Mode:                  preventative
Applied policy packs:  0
Stacks:                0
Accounts:              0
`, buf.String())
	assert.Equal(t, 1, c.createCalls)
	assert.Equal(t, 1, c.getCalls)
	assert.Equal(t, "acme", c.createdOrg)
	assert.Equal(t, "prod-policies", c.createdName)
	assert.Equal(t, "acme", c.getOrg)
	assert.Equal(t, "prod-policies", c.getName)
}

func TestPolicyGroupNew_JSONOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	args := policyGroupNewArgs{entityType: "stacks", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"name": "prod-policies",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "preventative",
		"stacks": [],
		"appliedPolicyPacks": [],
		"accounts": []
	}`, buf.String())
}

func TestPolicyGroupNew_CreateError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{createErr: errors.New("already exists")}
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{entityType: "stacks", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "creating policy group: already exists", err.Error())
	assert.Equal(t, 1, c.createCalls)
	assert.Equal(t, 0, c.getCalls)
}

func TestPolicyGroupNew_GetAfterCreateError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{getErr: errors.New("not found")}
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{entityType: "stacks", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "reading policy group after create: not found", err.Error())
	assert.Equal(t, 1, c.createCalls)
	assert.Equal(t, 1, c.getCalls)
}

func TestPolicyGroupNew_FactoryError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	err := runPolicyGroupNew(t.Context(), buf,
		failingPolicyGroupNewFactory(errors.New("not logged in")),
		"prod-policies", policyGroupNewArgs{
			entityType: "stacks", yes: true,
			outputFormat: defaultPolicyGroupGetOutputFormat(),
		})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}
