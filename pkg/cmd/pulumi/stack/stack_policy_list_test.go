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
	"encoding/json"
	"errors"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type mockStackPolicyListClient struct {
	groups   cloudapitype.AppListPolicyGroupsResponse
	packs    map[cloudapitype.PolicyGroupMode]cloudapitype.AppGetStackPolicyPacksResponse
	groupErr error
	packErr  error

	gotStacks []client.StackIdentifier
	gotModes  []cloudapitype.PolicyGroupMode
}

func (m *mockStackPolicyListClient) GetStackPolicyGroups(
	_ context.Context, stackID client.StackIdentifier,
) (cloudapitype.AppListPolicyGroupsResponse, error) {
	m.gotStacks = append(m.gotStacks, stackID)
	return m.groups, m.groupErr
}

func (m *mockStackPolicyListClient) GetStackPolicyPacksForMode(
	_ context.Context, stackID client.StackIdentifier, mode cloudapitype.PolicyGroupMode,
) (cloudapitype.AppGetStackPolicyPacksResponse, error) {
	m.gotStacks = append(m.gotStacks, stackID)
	m.gotModes = append(m.gotModes, mode)
	return m.packs[mode], m.packErr
}

var stackPolicyTestStackID = client.StackIdentifier{
	Owner:   "acme",
	Project: "web",
	Stack:   tokens.MustParseStackName("prod"),
}

func stubStackPolicyListFactory(c stackPolicyListClient) stackPolicyListClientFactory {
	return func(_ context.Context, _ string) (stackPolicyListClient, client.StackIdentifier, error) {
		return c, stackPolicyTestStackID, nil
	}
}

func sampleStackPolicyListClient() *mockStackPolicyListClient {
	return &mockStackPolicyListClient{
		groups: cloudapitype.AppListPolicyGroupsResponse{PolicyGroups: []cloudapitype.AppPolicyGroupSummary{
			{
				Name:                  "prod-policies",
				NumStacks:             4,
				EntityType:            cloudapitype.AppEntityTypeStacks,
				Mode:                  cloudapitype.PolicyGroupModePreventative,
				NumEnabledPolicyPacks: 1,
			},
			{
				Name:                  "default-audit",
				IsOrgDefault:          true,
				NumStacks:             10,
				NumAccounts:           2,
				EntityType:            cloudapitype.AppEntityTypeStacks,
				Mode:                  cloudapitype.PolicyGroupModeAudit,
				NumEnabledPolicyPacks: 1,
			},
		}},
		packs: map[cloudapitype.PolicyGroupMode]cloudapitype.AppGetStackPolicyPacksResponse{
			cloudapitype.PolicyGroupModePreventative: {RequiredPolicies: []cloudapitype.AppRequiredPolicy{{
				Name:         "aws-guardrails",
				DisplayName:  "AWS Guardrails",
				Version:      3,
				VersionTag:   "1.2.0",
				PackLocation: "https://example.com/signed-download",
				Config: map[string]*json.RawMessage{
					"all": new(json.RawMessage(`"mandatory"`)),
				},
				Environments: []string{"infra/policy-secrets"},
			}}},
			cloudapitype.PolicyGroupModeAudit: {RequiredPolicies: []cloudapitype.AppRequiredPolicy{{
				Name:        "tagging",
				DisplayName: "Tagging",
				Version:     1,
			}}},
		},
	}
}

func TestStackPolicyList_JSON(t *testing.T) {
	t.Parallel()

	c := sampleStackPolicyListClient()
	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}
	require.NoError(t, args.output.Set("json"))

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf, stubStackPolicyListFactory(c), args))

	assert.Equal(t, []cloudapitype.PolicyGroupMode{
		cloudapitype.PolicyGroupModePreventative, cloudapitype.PolicyGroupModeAudit,
	}, c.gotModes)
	for _, id := range c.gotStacks {
		assert.Equal(t, stackPolicyTestStackID, id)
	}
	assert.JSONEq(t, `{
		"stack": "acme/web/prod",
		"policyGroups": [
			{
				"name": "prod-policies", "isOrgDefault": false, "numStacks": 4,
				"entityType": "stacks", "mode": "preventative", "numEnabledPolicyPacks": 1
			},
			{
				"name": "default-audit", "isOrgDefault": true, "numStacks": 10, "numAccounts": 2,
				"entityType": "stacks", "mode": "audit", "numEnabledPolicyPacks": 1
			}
		],
		"policyPacks": [
			{
				"name": "aws-guardrails", "displayName": "AWS Guardrails", "version": 3, "versionTag": "1.2.0",
				"mode": "preventative", "config": {"all": "mandatory"}, "environments": ["infra/policy-secrets"]
			},
			{"name": "tagging", "displayName": "Tagging", "version": 1, "versionTag": "", "mode": "audit"}
		]
	}`, buf.String())
}

func TestStackPolicyList_JSON_Empty(t *testing.T) {
	t.Parallel()

	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}
	require.NoError(t, args.output.Set("json"))

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf,
		stubStackPolicyListFactory(&mockStackPolicyListClient{}), args))
	assert.JSONEq(t, `{"stack": "acme/web/prod", "policyGroups": [], "policyPacks": []}`, buf.String())
}

func TestStackPolicyList_Table(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf,
		stubStackPolicyListFactory(sampleStackPolicyListClient()),
		stackPolicyListArgs{output: defaultStackPolicyListOutput()}))

	assert.Equal(t, `POLICY GROUP   MODE          DEFAULT
prod-policies  preventative  N
default-audit  audit         Y

POLICY PACK     VERSION  MODE
aws-guardrails  1.2.0    preventative
tagging         v1       audit
`, buf.String())
}

func TestStackPolicyList_Table_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf,
		stubStackPolicyListFactory(&mockStackPolicyListClient{}),
		stackPolicyListArgs{output: defaultStackPolicyListOutput()}))

	assert.Equal(t, `No Policy Groups apply to stack acme/web/prod.

No Policy Packs apply to stack acme/web/prod.
`, buf.String())
}

func TestStackPolicyList_Errors(t *testing.T) {
	t.Parallel()

	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}

	err := runStackPolicyList(t.Context(), &bytes.Buffer{},
		stubStackPolicyListFactory(&mockStackPolicyListClient{groupErr: errors.New("groups boom")}), args)
	assert.EqualError(t, err, "groups boom")

	err = runStackPolicyList(t.Context(), &bytes.Buffer{},
		stubStackPolicyListFactory(&mockStackPolicyListClient{packErr: errors.New("packs boom")}), args)
	assert.EqualError(t, err, "packs boom")

	err = runStackPolicyList(t.Context(), &bytes.Buffer{},
		func(context.Context, string) (stackPolicyListClient, client.StackIdentifier, error) {
			return nil, client.StackIdentifier{}, errors.New("not logged in")
		}, args)
	assert.EqualError(t, err, "not logged in")
}

func TestStackPolicyList_Flags(t *testing.T) {
	t.Parallel()

	var gotStack string
	c := sampleStackPolicyListClient()
	cmd := newStackPolicyListCmdWith(func(_ context.Context, stackFlag string) (
		stackPolicyListClient, client.StackIdentifier, error,
	) {
		gotStack = stackFlag
		return c, stackPolicyTestStackID, nil
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"-s", "acme/web/prod", "--output", "json"})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, "acme/web/prod", gotStack)
	var out stackPolicyListResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.Equal(t, "acme/web/prod", out.Stack)
}
