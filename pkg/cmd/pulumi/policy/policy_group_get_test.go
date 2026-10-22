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

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPolicyGroupGetClient stubs policyGroupGetClient. It returns a fixed
// response (or error) and records the arguments it was called with.
type mockPolicyGroupGetClient struct {
	resp     cloudapitype.PolicyGroup
	err      error
	gotOrg   string
	gotGroup string
}

func (m *mockPolicyGroupGetClient) GetPolicyGroup(
	_ context.Context, org, group string,
) (cloudapitype.PolicyGroup, error) {
	m.gotOrg = org
	m.gotGroup = group
	if m.err != nil {
		return cloudapitype.PolicyGroup{}, m.err
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

func samplePolicyGroupResponse() cloudapitype.PolicyGroup {
	return cloudapitype.PolicyGroup{
		Name:         "prod-policies",
		IsOrgDefault: false,
		EntityType:   cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         cloudapitype.PolicyGroupModePreventative,
		Stacks: []cloudapitype.AppPulumiStackReference{
			{Name: "prod", RoutingProject: "web"},
			{Name: "standalone"},
		},
		AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{
			{
				Name:         "aws-guardrails",
				DisplayName:  "AWS Guardrails",
				Version:      3,
				VersionTag:   "1.2.0",
				Config:       map[string]*json.RawMessage{"s3-no-public": new(json.RawMessage(`{"enforcementLevel":"mandatory"}`))},
				Environments: []string{"infra/policy-secrets"},
			},
			{Name: "tagging", Version: 1},
		},
		Accounts:    []string{"prod-aws"},
		AgentPoolID: new("pool-1"),
	}
}

func TestPolicyGroupGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: samplePolicyGroupResponse()}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies",
		policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  prod-policies
Org default:           no
Mode:                  preventative
Agent pool:            pool-1
Applied policy packs:  2
  - aws-guardrails (1.2.0)
  - tagging (v1)
Stacks:                2
  - web/prod
  - standalone
Accounts:              1
  - prod-aws
`, buf.String())
	assert.Equal(t, "acme", c.gotOrg)
	assert.Equal(t, "prod-policies", c.gotGroup)
}

func TestPolicyGroupGet_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	resp := cloudapitype.PolicyGroup{
		Name:         "default-policy-group",
		IsOrgDefault: true,
		EntityType:   cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         cloudapitype.PolicyGroupModeAudit,
	}
	c := &mockPolicyGroupGetClient{resp: resp}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "default-policy-group",
		policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  default-policy-group
Org default:           yes
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
	args := policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"name": "prod-policies",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "preventative",
		"stacks": [
			{"name": "prod", "routingProject": "web"},
			{"name": "standalone", "routingProject": ""}
		],
		"appliedPolicyPacks": [
			{
				"name": "aws-guardrails",
				"displayName": "AWS Guardrails",
				"version": 3,
				"versionTag": "1.2.0",
				"config": {"s3-no-public": {"enforcementLevel": "mandatory"}},
				"environments": ["infra/policy-secrets"]
			},
			{"name": "tagging", "displayName": "", "version": 1, "versionTag": ""}
		],
		"accounts": ["prod-aws"],
		"agentPoolId": "pool-1"
	}`, buf.String())
}

func TestPolicyGroupGet_JSONOutput_NilSlicesNormalized(t *testing.T) {
	t.Parallel()

	resp := cloudapitype.PolicyGroup{
		Name:         "bare",
		IsOrgDefault: true,
		EntityType:   cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         cloudapitype.PolicyGroupModeAudit,
	}

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: resp}
	args := policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "bare", args)
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

func TestPolicyGroupGet_JSONOutput_MatchRule(t *testing.T) {
	t.Parallel()

	group := `{
		"name": "tagged",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "audit",
		"stacks": [],
		"appliedPolicyPacks": [],
		"accounts": [],
		"stackMatchRule": {
			"__type": "PermissionExpressionHasTag",
			"context": {"__type": "PermissionExpressionStack"},
			"key": "env"
		}
	}`
	var resp cloudapitype.PolicyGroup
	require.NoError(t, json.Unmarshal([]byte(group), &resp))

	var buf bytes.Buffer
	require.NoError(t, renderPolicyGroupGetJSON(&buf, resp))
	assert.JSONEq(t, group, buf.String())
}

func TestPolicyGroupGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{err: errors.New("not found")}
	err := runPolicyGroupGet(t.Context(), &buf,
		stubPolicyGroupGetFactory(c, "acme"), "missing",
		policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "getting policy group: not found", err.Error())
}

func TestPolicyGroupGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runPolicyGroupGet(t.Context(), &buf,
		failingPolicyGroupGetFactory(errors.New("not logged in")),
		"prod-policies", policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}

func TestPolicyGroupGet_DefaultOutput_MatchRules(t *testing.T) {
	t.Parallel()

	group := `{
		"name": "tagged",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "audit",
		"stacks": [{"name": "prod", "routingProject": "web"}],
		"appliedPolicyPacks": [],
		"accounts": [],
		"stackMatchRule": ` + binaryJSON("Or",
		binaryJSON("And", tagEqualsJSON(stackCtxJSON, "env", "prod"), notJSON(hasTagJSON(stackCtxJSON, "temp"))),
		hasTagJSON(stackCtxJSON, "owner")) + `,
		"accountMatchRule": {
			"__type": "PermissionExpressionIn",
			"target": {"__type": "PermissionExpressionTag", "context": ` + accountCtxJSON + `, "key": "env"},
			"values": {"__type": "PermissionSetExpressionString", "values": ["prod"]}
		}
	}`
	var resp cloudapitype.PolicyGroup
	require.NoError(t, json.Unmarshal([]byte(group), &resp))

	var buf bytes.Buffer
	require.NoError(t, renderPolicyGroupGetText(&buf, resp))
	assert.Equal(t, `Name:                  tagged
Org default:           no
Mode:                  audit
Applied policy packs:  0
Stack rule:            env=prod,!temp OR owner
Stacks:                1
  - web/prod
Account rule:          {"__type":"PermissionExpressionIn",`+
		`"target":{"__type":"PermissionExpressionTag",`+
		`"context":{"__type":"PermissionExpressionInsightsAccount"},"key":"env"},`+
		`"values":{"__type":"PermissionSetExpressionString","values":["prod"]}}
Accounts:              0
`, buf.String())
}
