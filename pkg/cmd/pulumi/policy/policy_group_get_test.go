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

// mockPolicyGroupGetClient stubs policyGroupGetClient. It returns a fixed
// response (or error) and records the arguments it was called with.
type mockPolicyGroupGetClient struct {
	resp     apitype.GetPolicyGroupResponse
	err      error
	gotOrg   string
	gotGroup string
}

func (m *mockPolicyGroupGetClient) GetPolicyGroup(
	_ context.Context, org, group string,
) (apitype.GetPolicyGroupResponse, error) {
	m.gotOrg = org
	m.gotGroup = group
	if m.err != nil {
		return apitype.GetPolicyGroupResponse{}, m.err
	}
	return m.resp, nil
}

func stubPolicyGroupGetFactory(c policyGroupGetClient, org string) policyGroupGetClientFactory {
	return func(_ context.Context, _ string) (policyGroupGetClient, string, error) {
		return c, org, nil
	}
}

func failingPolicyGroupGetFactory(err error) policyGroupGetClientFactory {
	return func(_ context.Context, _ string) (policyGroupGetClient, string, error) {
		return nil, "", err
	}
}

func samplePolicyGroupResponse() apitype.GetPolicyGroupResponse {
	return apitype.GetPolicyGroupResponse{
		Name:         "prod-policies",
		IsOrgDefault: false,
		EntityType:   apitype.Stacks,
		Mode:         apitype.PolicyGroupModePreventative,
		Stacks: []apitype.PulumiStackReference{
			{Name: "prod", RoutingProject: "web"},
			{Name: "staging", RoutingProject: "web"},
		},
		AppliedPolicyPacks: []apitype.PolicyPackMetadata{
			{Name: "aws-guardrails", DisplayName: "AWS Guardrails", Version: 3, VersionTag: "1.2.0"},
			{Name: "tagging", DisplayName: "Tagging", Version: 1, VersionTag: ""},
		},
		Accounts:    []string{},
		AgentPoolID: "pool-1",
	}
}

func TestPolicyGroupGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: samplePolicyGroupResponse()}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies", policyGroupGetArgs{})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  prod-policies
Org default:           no
Entity type:           stacks
Mode:                  preventative
Agent pool:            pool-1
Applied policy packs:  2
  - aws-guardrails (1.2.0)
  - tagging (v1)
Stacks:                2
  - web/prod
  - web/staging
Accounts:              0
`, buf.String())
	assert.Equal(t, "acme", c.gotOrg)
	assert.Equal(t, "prod-policies", c.gotGroup)
}

func TestPolicyGroupGet_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	resp := apitype.GetPolicyGroupResponse{
		Name:         "default-policy-group",
		IsOrgDefault: true,
		EntityType:   apitype.Stacks,
		Mode:         apitype.PolicyGroupModeAudit,
	}
	c := &mockPolicyGroupGetClient{resp: resp}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "default-policy-group", policyGroupGetArgs{})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  default-policy-group
Org default:           yes
Entity type:           stacks
Mode:                  audit
Applied policy packs:  0
Stacks:                0
Accounts:              0
`, buf.String())
}

func TestPolicyGroupGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: samplePolicyGroupResponse()}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies",
		policyGroupGetArgs{output: "json"})
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"name": "prod-policies",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "preventative",
		"stacks": [
			{"name": "prod", "routingProject": "web"},
			{"name": "staging", "routingProject": "web"}
		],
		"appliedPolicyPacks": [
			{"name": "aws-guardrails", "displayName": "AWS Guardrails", "version": 3, "versionTag": "1.2.0"},
			{"name": "tagging", "displayName": "Tagging", "version": 1, "versionTag": ""}
		],
		"accounts": [],
		"agentPoolId": "pool-1"
	}`, buf.String())
}

func TestPolicyGroupGet_JSONOutput_NilSlicesNormalized(t *testing.T) {
	t.Parallel()

	resp := apitype.GetPolicyGroupResponse{
		Name:         "bare",
		IsOrgDefault: true,
		EntityType:   apitype.Stacks,
		Mode:         apitype.PolicyGroupModeAudit,
	}

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: resp}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "bare",
		policyGroupGetArgs{output: "json"})
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"name": "bare",
		"isOrgDefault": true,
		"entityType": "stacks",
		"mode": "audit",
		"stacks": [],
		"appliedPolicyPacks": [],
		"accounts": []
	}`, buf.String())
}

func TestPolicyGroupGet_InvalidOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: samplePolicyGroupResponse()}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies",
		policyGroupGetArgs{output: "yaml"})
	require.Error(t, err)
	assert.Equal(t,
		`invalid --output value "yaml" (must be 'default' or 'json')`,
		err.Error())
	// Validation must run before the API call.
	assert.Equal(t, "", c.gotOrg)
	assert.Equal(t, "", c.gotGroup)
}

func TestPolicyGroupGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{err: errors.New("not found")}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "missing", policyGroupGetArgs{})
	require.Error(t, err)
	assert.Equal(t, "getting policy group: not found", err.Error())
}

func TestPolicyGroupGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runPolicyGroupGet(t.Context(), &buf,
		failingPolicyGroupGetFactory(errors.New("not logged in")),
		"prod-policies", policyGroupGetArgs{})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}
