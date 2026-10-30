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
	"fmt"
	"io"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
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

func (m *mockPolicyGroupGetClient) ListPolicyGroupStacks(
	context.Context, string, string, client.ListPolicyGroupMembersOptions,
) (cloudapitype.ListPolicyGroupStacksResponse, error) {
	panic("unexpected ListPolicyGroupStacks call")
}

func (m *mockPolicyGroupGetClient) ListPolicyGroupAccounts(
	context.Context, string, string, client.ListPolicyGroupMembersOptions,
) (cloudapitype.ListPolicyGroupAccountsResponse, error) {
	panic("unexpected ListPolicyGroupAccounts call")
}

// mockPolicyGroupMembersClient serves member pages in order and records the
// options of each request.
type mockPolicyGroupMembersClient struct {
	mockPolicyGroupGetClient
	stackPages   []cloudapitype.ListPolicyGroupStacksResponse
	accountPages []cloudapitype.ListPolicyGroupAccountsResponse
	err          error
	gotOpts      []client.ListPolicyGroupMembersOptions
}

func (m *mockPolicyGroupMembersClient) ListPolicyGroupStacks(
	_ context.Context, org, group string, opts client.ListPolicyGroupMembersOptions,
) (cloudapitype.ListPolicyGroupStacksResponse, error) {
	m.gotOrg, m.gotGroup = org, group
	m.gotOpts = append(m.gotOpts, opts)
	if m.err != nil {
		return cloudapitype.ListPolicyGroupStacksResponse{}, m.err
	}
	return m.stackPages[len(m.gotOpts)-1], nil
}

func (m *mockPolicyGroupMembersClient) ListPolicyGroupAccounts(
	_ context.Context, org, group string, opts client.ListPolicyGroupMembersOptions,
) (cloudapitype.ListPolicyGroupAccountsResponse, error) {
	m.gotOrg, m.gotGroup = org, group
	m.gotOpts = append(m.gotOpts, opts)
	if m.err != nil {
		return cloudapitype.ListPolicyGroupAccountsResponse{}, m.err
	}
	return m.accountPages[len(m.gotOpts)-1], nil
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
		EntityType:   apitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         apitype.PolicyGroupModePreventative,
		Stacks: []apitype.PulumiStackReference{
			{Name: "prod", RoutingProject: "web"},
			{Name: "standalone"},
		},
		AppliedPolicyPacks: []apitype.PolicyPackMetadata{
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
	err := runPolicyGroupGet(t.Context(), &buf, io.Discard,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies",
		policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  prod-policies
Org default:           no
Mode:                  preventative
Agent pool:            pool-1
Applied policy packs:  2
  - aws-guardrails (1.2.0), 1 configured policy
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
	resp := apitype.GetPolicyGroupResponse{
		Name:         "default-policy-group",
		IsOrgDefault: true,
		EntityType:   apitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         apitype.PolicyGroupModeAudit,
	}
	c := &mockPolicyGroupGetClient{resp: resp}
	err := runPolicyGroupGet(t.Context(), &buf, io.Discard,
		stubPolicyGroupGetFactory(c, "acme"), "default-policy-group",
		policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()})
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
	args := policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupGet(t.Context(), &buf, io.Discard,
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

	resp := apitype.GetPolicyGroupResponse{
		Name:         "bare",
		IsOrgDefault: true,
		EntityType:   apitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         apitype.PolicyGroupModeAudit,
	}

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{resp: resp}
	args := policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupGet(t.Context(), &buf, io.Discard,
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
	var resp apitype.GetPolicyGroupResponse
	require.NoError(t, json.Unmarshal([]byte(group), &resp))

	var buf bytes.Buffer
	require.NoError(t, renderPolicyGroupGetJSON(&buf, resp))
	assert.JSONEq(t, group, buf.String())
}

func TestPolicyGroupGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockPolicyGroupGetClient{err: errors.New("not found")}
	err := runPolicyGroupGet(t.Context(), &buf, io.Discard,
		stubPolicyGroupGetFactory(c, "acme"), "missing",
		policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()})
	require.Error(t, err)
	assert.Equal(t, "getting policy group: not found", err.Error())
}

func TestPolicyGroupGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runPolicyGroupGet(t.Context(), &buf, io.Discard,
		failingPolicyGroupGetFactory(errors.New("not logged in")),
		"prod-policies", policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}

func TestPolicyGroupGet_AgentPoolPointer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		agentPoolID *string
		wantText    bool
	}{
		{name: "absent", agentPoolID: nil, wantText: false},
		{name: "present", agentPoolID: new("pool-1"), wantText: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := samplePolicyGroupResponse()
			resp.AgentPoolID = tc.agentPoolID

			var text bytes.Buffer
			err := runPolicyGroupGet(t.Context(), &text, io.Discard,
				stubPolicyGroupGetFactory(&mockPolicyGroupGetClient{resp: resp}, "acme"), "prod-policies",
				policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()})
			require.NoError(t, err)

			var jsonOut bytes.Buffer
			args := policyGroupGetArgs{outputFormat: defaultPolicyGroupGetRenderers()}
			require.NoError(t, args.outputFormat.Set("json"))
			err = runPolicyGroupGet(t.Context(), &jsonOut, io.Discard,
				stubPolicyGroupGetFactory(&mockPolicyGroupGetClient{resp: resp}, "acme"), "prod-policies", args)
			require.NoError(t, err)

			if tc.wantText {
				assert.Contains(t, text.String(), "Agent pool:            pool-1\n")
				assert.Contains(t, jsonOut.String(), `"agentPoolId": "pool-1"`)
			} else {
				assert.NotContains(t, text.String(), "Agent pool:")
				assert.NotContains(t, jsonOut.String(), "agentPoolId")
			}
		})
	}
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
	var resp apitype.GetPolicyGroupResponse
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

func stackPage(token string, names ...string) cloudapitype.ListPolicyGroupStacksResponse {
	resp := cloudapitype.ListPolicyGroupStacksResponse{Stacks: []cloudapitype.PolicyGroupStack{}}
	for _, n := range names {
		resp.Stacks = append(resp.Stacks, cloudapitype.PolicyGroupStack{
			Name: n, RoutingProject: "web", Source: cloudapitype.PolicyGroupMemberSourceRule,
		})
	}
	if token != "" {
		resp.ContinuationToken = new(token)
	}
	return resp
}

func stackNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("s%d", i)
	}
	return names
}

func runPolicyGroupGetMembers(
	t *testing.T, c policyGroupGetClient, args policyGroupGetArgs, format string,
) (string, string, error) {
	t.Helper()
	args.outputFormat = defaultPolicyGroupGetRenderers()
	if format != "" {
		require.NoError(t, args.outputFormat.Set(format))
	}
	var out, errOut bytes.Buffer
	err := runPolicyGroupGet(t.Context(), &out, &errOut,
		stubPolicyGroupGetFactory(c, "acme"), "prod-policies", args)
	return out.String(), errOut.String(), err
}

func TestPolicyGroupGet_Stacks_Table(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{{
		Stacks: []cloudapitype.PolicyGroupStack{
			{
				Name: "prod", RoutingProject: "infra", Source: cloudapitype.PolicyGroupMemberSourceExplicit,
				Tags: map[string]string{"team": "platform", "env": "prod"},
			},
			{Name: "standalone", Source: cloudapitype.PolicyGroupMemberSourceRule},
		},
	}}}
	out, errOut, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true}, "")
	require.NoError(t, err)

	for _, want := range []string{
		"NAME", "SOURCE", "TAGS",
		"infra/prod", "explicit", "env=prod, team=platform",
		"standalone", "rule",
	} {
		assert.Contains(t, out, want)
	}
	assert.Empty(t, errOut)
	assert.Equal(t, "acme", c.gotOrg)
	assert.Equal(t, "prod-policies", c.gotGroup)
	assert.Equal(t, []client.ListPolicyGroupMembersOptions{{Count: 100}}, c.gotOpts)
}

func TestPolicyGroupGet_Stacks_Empty(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{{}}}
	out, _, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true}, "")
	require.NoError(t, err)
	assert.Equal(t, "No stacks found.\n", out)

	c = &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{{}}}
	out, _, err = runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true}, "json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"stacks": []}`, out)
}

func TestPolicyGroupGet_Stacks_DefaultCountHintsAtMore(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{
		stackPage("t1", stackNames(100)...),
	}}
	out, errOut, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true}, "json")
	require.NoError(t, err)

	var resp cloudapitype.ListPolicyGroupStacksResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Stacks, 100)
	assert.NotContains(t, out, "continuationToken")
	assert.Equal(t, "More stacks available; re-run with --count <N> or --all to fetch more.\n", errOut)
	require.Len(t, c.gotOpts, 1)
}

func TestPolicyGroupGet_Stacks_CountSpansPages(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{
		stackPage("t1", stackNames(500)...),
		stackPage("t2", stackNames(200)...),
	}}
	out, errOut, err := runPolicyGroupGetMembers(t, c,
		policyGroupGetArgs{stacks: true, count: 700, source: "rule"}, "json")
	require.NoError(t, err)

	var resp cloudapitype.ListPolicyGroupStacksResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Stacks, 700)
	assert.Equal(t, []client.ListPolicyGroupMembersOptions{
		{Source: cloudapitype.PolicyGroupMemberSourceRule, Count: 500},
		{Source: cloudapitype.PolicyGroupMemberSourceRule, Count: 200, ContinuationToken: new("t1")},
	}, c.gotOpts)
	assert.Contains(t, errOut, "More stacks available")
}

func TestPolicyGroupGet_Stacks_AllFollowsEveryPage(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{
		stackPage("t1", stackNames(500)...),
		stackPage("t2", stackNames(500)...),
		stackPage("", stackNames(3)...),
	}}
	out, errOut, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true, all: true}, "json")
	require.NoError(t, err)

	var resp cloudapitype.ListPolicyGroupStacksResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Stacks, 1003)
	assert.Empty(t, errOut)
	assert.Equal(t, []client.ListPolicyGroupMembersOptions{
		{Count: 500},
		{Count: 500, ContinuationToken: new("t1")},
		{Count: 500, ContinuationToken: new("t2")},
	}, c.gotOpts)
}

func TestPolicyGroupGet_Stacks_StopsOnEmptyPageWithToken(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{stackPages: []cloudapitype.ListPolicyGroupStacksResponse{
		stackPage("t1", "a"),
		stackPage("t2"),
	}}
	out, errOut, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true, all: true}, "json")
	require.NoError(t, err)
	require.Len(t, c.gotOpts, 2)
	assert.Contains(t, out, `"name": "a"`)
	assert.Empty(t, errOut)
}

func TestPolicyGroupGet_Stacks_PageError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupMembersClient{err: errors.New("listing policy group stacks: boom")}
	out, _, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{stacks: true}, "")
	require.EqualError(t, err, "listing policy group stacks: boom")
	assert.Empty(t, out)
}

func TestPolicyGroupGet_Accounts(t *testing.T) {
	t.Parallel()

	page := cloudapitype.ListPolicyGroupAccountsResponse{
		Accounts: []cloudapitype.PolicyGroupAccount{
			{ID: "acc-1", Name: "aws-prod", Source: cloudapitype.PolicyGroupMemberSourceExplicit, Tags: map[string]string{}},
			{
				ID: "acc-2", Name: "aws-payments", Source: cloudapitype.PolicyGroupMemberSourceRule,
				Tags: map[string]string{"env": "prod"},
			},
		},
	}

	c := &mockPolicyGroupMembersClient{accountPages: []cloudapitype.ListPolicyGroupAccountsResponse{page}}
	out, errOut, err := runPolicyGroupGetMembers(t, c, policyGroupGetArgs{accounts: true}, "")
	require.NoError(t, err)
	for _, want := range []string{
		"NAME", "SOURCE", "TAGS",
		"aws-prod", "explicit",
		"aws-payments", "rule", "env=prod",
	} {
		assert.Contains(t, out, want)
	}
	assert.Empty(t, errOut)

	c = &mockPolicyGroupMembersClient{accountPages: []cloudapitype.ListPolicyGroupAccountsResponse{page}}
	out, _, err = runPolicyGroupGetMembers(t, c, policyGroupGetArgs{accounts: true}, "json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"accounts": [
		{"id": "acc-1", "name": "aws-prod", "source": "explicit", "tags": {}},
		{"id": "acc-2", "name": "aws-payments", "source": "rule", "tags": {"env": "prod"}}
	]}`, out)
}

func TestPolicyGroupGet_MemberFlagValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    policyGroupGetArgs
		wantErr string
	}{
		{"source without member flag", policyGroupGetArgs{source: "rule"}, "--source requires --stacks or --accounts"},
		{"count without member flag", policyGroupGetArgs{count: 5}, "--count requires --stacks or --accounts"},
		{"all without member flag", policyGroupGetArgs{all: true}, "--all requires --stacks or --accounts"},
		{"negative count", policyGroupGetArgs{stacks: true, count: -1}, "--count must be non-negative, got -1"},
		{
			"unknown source",
			policyGroupGetArgs{accounts: true, source: "tag"},
			`--source must be one of explicit or rule, got "tag"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := runPolicyGroupGetMembers(t, &mockPolicyGroupMembersClient{}, tt.args, "")
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestPolicyGroupGet_MemberFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()

	for _, flags := range [][]string{{"--stacks", "--accounts"}, {"--stacks", "--count", "5", "--all"}} {
		cmd := newPolicyGroupGetCmdWith(stubPolicyGroupGetFactory(&mockPolicyGroupMembersClient{}, "acme"))
		cmd.SetArgs(append([]string{"prod-policies"}, flags...))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		require.ErrorContains(t, cmd.Execute(), "none of the others can be")
	}
}

func raw(s string) *json.RawMessage {
	return new(json.RawMessage(s))
}

func renderGroupText(t *testing.T, showConfig bool, packs ...apitype.PolicyPackMetadata) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, writePolicyGroupText(&buf, apitype.GetPolicyGroupResponse{
		Name:               "g",
		Mode:               apitype.PolicyGroupModeAudit,
		AppliedPolicyPacks: packs,
	}, showConfig))
	return buf.String()
}

func TestPolicyGroupGet_PackConfigCount(t *testing.T) {
	t.Parallel()

	out := renderGroupText(t, false,
		apitype.PolicyPackMetadata{Name: "none", VersionTag: "1.0.0"},
		apitype.PolicyPackMetadata{Name: "empty", VersionTag: "1.0.0", Config: map[string]*json.RawMessage{}},
		apitype.PolicyPackMetadata{Name: "one", VersionTag: "2.0.0",
			Config: map[string]*json.RawMessage{"a": raw(`"mandatory"`)}},
		apitype.PolicyPackMetadata{Name: "two", VersionTag: "2.0.0",
			Config: map[string]*json.RawMessage{"a": raw(`"mandatory"`), "b": raw(`{"minLength":1}`)}},
		apitype.PolicyPackMetadata{Name: "allonly", VersionTag: "1.2.1",
			Config: map[string]*json.RawMessage{"all": raw(`"advisory"`)}},
		apitype.PolicyPackMetadata{Name: "allplus", VersionTag: "1.2.1",
			Config: map[string]*json.RawMessage{
				"all": raw(`"advisory"`), "a": raw(`"mandatory"`), "b": raw(`"mandatory"`), "c": raw(`"mandatory"`),
			}},
	)
	assert.Contains(t, out, "  - none (1.0.0)\n")
	assert.Contains(t, out, "  - empty (1.0.0)\n")
	assert.Contains(t, out, "  - one (2.0.0), 1 configured policy\n")
	assert.Contains(t, out, "  - two (2.0.0), 2 configured policies\n")
	assert.Contains(t, out, "  - allonly (1.2.1), settings for all policies\n")
	assert.Contains(t, out, "  - allplus (1.2.1), 3 configured policies, settings for all policies\n")
	assert.NotContains(t, out, "\n      ")
}

func TestPolicyGroupGet_ShowConfig(t *testing.T) {
	t.Parallel()

	out := renderGroupText(t, true,
		apitype.PolicyPackMetadata{Name: "bare", VersionTag: "1.0.0"},
		apitype.PolicyPackMetadata{Name: "pack", VersionTag: "2.0.0",
			Config: map[string]*json.RawMessage{
				"zeta":         raw(`"mandatory"`),
				"all":          raw(`{"enforcementLevel":"advisory"}`),
				"level-only":   raw(`{"enforcementLevel":"mandatory"}`),
				"settings":     raw(`{"minLength":24}`),
				"both":         raw(`{"minLength":24,"enforcementLevel":"mandatory"}`),
				"nested":       raw(`{"tags":["a","b"]}`),
				"empty-object": raw(`{}`),
				"number":       raw(`5`),
			}},
		apitype.PolicyPackMetadata{Name: "last", VersionTag: "3.0.0"},
	)
	assert.Contains(t, out, `  - bare (1.0.0)
  - pack (2.0.0), 7 configured policies, settings for all policies
      all: advisory
      both: mandatory, minLength=24
      empty-object: {}
      level-only: mandatory
      nested: tags=["a","b"]
      number: 5
      settings: minLength=24
      zeta: mandatory
  - last (3.0.0)
`)
}

func TestPolicyGroupGet_ShowConfigViaCommand(t *testing.T) {
	t.Parallel()

	resp := samplePolicyGroupResponse()
	cmd := newPolicyGroupGetCmdWith(stubPolicyGroupGetFactory(&mockPolicyGroupGetClient{resp: resp}, "acme"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"prod-policies", "--show-config"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "  - aws-guardrails (1.2.0), 1 configured policy\n      s3-no-public: mandatory\n")
}

func TestPolicyGroupGet_ShowConfigJSONUnchanged(t *testing.T) {
	t.Parallel()

	resp := samplePolicyGroupResponse()
	run := func(extra ...string) string {
		cmd := newPolicyGroupGetCmdWith(stubPolicyGroupGetFactory(&mockPolicyGroupGetClient{resp: resp}, "acme"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"prod-policies", "--output", "json"}, extra...))
		require.NoError(t, cmd.Execute())
		return out.String()
	}
	with := run("--show-config")
	assert.Equal(t, run(), with)
	assert.Contains(t, with, `"s3-no-public"`)
}

func TestPolicyGroupGet_ShowConfigMutuallyExclusive(t *testing.T) {
	t.Parallel()

	for _, other := range []string{"--stacks", "--accounts"} {
		cmd := newPolicyGroupGetCmdWith(stubPolicyGroupGetFactory(&mockPolicyGroupMembersClient{}, "acme"))
		cmd.SetArgs([]string{"prod-policies", "--show-config", other})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		require.ErrorContains(t, cmd.Execute(), "none of the others can be")
	}
}
