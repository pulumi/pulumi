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

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockPolicyGroupEditClient records each BatchUpdatePolicyGroup call and
// returns getResp / getErr from GetPolicyGroup. updateErr forces
// BatchUpdatePolicyGroup to fail.
type mockPolicyGroupEditClient struct {
	updates      [][]cloudapitype.UpdatePolicyGroupRequest
	updateGroups []string
	updateErr    error

	getResp     apitype.GetPolicyGroupResponse
	getErr      error
	getErrOn    int // which Get call to fail (1-indexed)
	getCalls    int
	gotGetOrg   string
	gotGetGroup string
}

func (m *mockPolicyGroupEditClient) BatchUpdatePolicyGroup(
	_ context.Context, _, policyGroup string, reqs []cloudapitype.UpdatePolicyGroupRequest,
) error {
	m.updates = append(m.updates, reqs)
	m.updateGroups = append(m.updateGroups, policyGroup)
	return m.updateErr
}

func (m *mockPolicyGroupEditClient) GetPolicyGroup(
	_ context.Context, org, policyGroup string,
) (apitype.GetPolicyGroupResponse, error) {
	m.getCalls++
	m.gotGetOrg = org
	m.gotGetGroup = policyGroup
	if m.getErrOn > 0 && m.getCalls == m.getErrOn {
		return apitype.GetPolicyGroupResponse{}, m.getErr
	}
	if m.getErrOn == 0 && m.getErr != nil {
		return apitype.GetPolicyGroupResponse{}, m.getErr
	}
	return m.getResp, nil
}

func stubPolicyGroupEditFactory(c policyGroupEditClient, org string) policyGroupEditClientFactory {
	return func(_ context.Context, _ string) (policyGroupEditClient, string, error) {
		return c, org, nil
	}
}

func failingPolicyGroupEditFactory(err error) policyGroupEditClientFactory {
	return func(_ context.Context, _ string) (policyGroupEditClient, string, error) {
		return nil, "", err
	}
}

func TestPolicyGroupEdit_RenameOnly_DefaultOutput(t *testing.T) {
	t.Parallel()

	resp := apitype.GetPolicyGroupResponse{
		Name:         "production",
		IsOrgDefault: false,
		EntityType:   apitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         apitype.PolicyGroupModeAudit,
	}
	c := &mockPolicyGroupEditClient{getResp: resp}

	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			rename:       "production",
			changed:      map[string]bool{"name": true},
		})
	require.NoError(t, err)

	assert.Equal(t, 1, c.getCalls)
	assert.Equal(t, [][]cloudapitype.UpdatePolicyGroupRequest{
		{policyGroupUpdate(apitype.UpdatePolicyGroupRequest{NewName: new("production")})},
	}, c.updates)
	assert.Equal(t, []string{"prod-policies"}, c.updateGroups)
	assert.Equal(t, "acme", c.gotGetOrg)
	assert.Equal(t, "production", c.gotGetGroup)
	assert.Equal(t, `Name:                  production
Org default:           no
Mode:                  audit
Applied policy packs:  0
Stacks:                0
Accounts:              0
`, buf.String())
}

func TestPolicyGroupEdit_AddsAndRemoves_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: apitype.GetPolicyGroupResponse{
		Name:       "production",
		EntityType: apitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:       apitype.PolicyGroupModePreventative,
	}}

	args := policyGroupEditArgs{
		outputFormat:          defaultPolicyGroupGetOutputFormat(),
		rename:                "production",
		addStack:              []string{"web/prod", "standalone"},
		removeStack:           []string{"web/legacy"},
		addPolicyPack:         []string{"aws-guardrails@3", "tagging"},
		removePolicyPack:      []string{"old-pack@stable", "unpinned"},
		addInsightsAccount:    []string{"acct-2"},
		removeInsightsAccount: []string{"acct-1"},
		changed: map[string]bool{
			"name": true, "add-stack": true, "remove-stack": true,
			"add-policy-pack": true, "remove-policy-pack": true,
			"add-insights-account": true, "remove-insights-account": true,
		},
	}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", args)
	require.NoError(t, err)

	assert.Equal(t, [][]cloudapitype.UpdatePolicyGroupRequest{{
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{NewName: new("production")}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			AddStack: &apitype.PulumiStackReference{Name: "prod", RoutingProject: "web"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			AddStack: &apitype.PulumiStackReference{Name: "standalone"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			RemoveStack: &apitype.PulumiStackReference{Name: "legacy", RoutingProject: "web"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			AddInsightsAccount: &apitype.InsightsAccountReference{Name: "acct-2"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			RemoveInsightsAccount: &apitype.InsightsAccountReference{Name: "acct-1"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			AddPolicyPack: &apitype.PolicyPackMetadata{Name: "aws-guardrails", Version: 3},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			AddPolicyPack: &apitype.PolicyPackMetadata{Name: "tagging"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			RemovePolicyPack: &apitype.PolicyPackMetadata{Name: "old-pack", VersionTag: "stable"},
		}),
		policyGroupUpdate(apitype.UpdatePolicyGroupRequest{
			RemovePolicyPack: &apitype.PolicyPackMetadata{Name: "unpinned"},
		}),
	}}, c.updates)

	// The batch addresses the original group name; the post-edit GET uses
	// the new name.
	assert.Equal(t, []string{"prod-policies"}, c.updateGroups)
	assert.Equal(t, 1, c.getCalls)
	assert.Equal(t, "production", c.gotGetGroup)
	assert.JSONEq(t, `{
		"name": "production",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "preventative",
		"stacks": [],
		"appliedPolicyPacks": [],
		"accounts": []
	}`, buf.String())
}

func TestPolicyGroupEdit_NoFlagsChanged(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			changed:      map[string]bool{},
		})
	require.Error(t, err)
	assert.Equal(t,
		"no changes specified; pass at least one of --name, --add-stack, --remove-stack, "+
			"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account, "+
			"--stack-rule, --account-rule",
		err.Error())
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_UpdateError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{updateErr: errors.New("conflict")}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			rename:       "production",
			changed:      map[string]bool{"name": true},
		})
	require.Error(t, err)
	assert.Equal(t, "conflict", err.Error())
	// The post-edit render is skipped on PATCH failure.
	assert.Equal(t, 0, c.getCalls)
}

func TestPolicyGroupEdit_GetAfterEditError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getErr: errors.New("boom"), getErrOn: 1}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			rename:       "production",
			changed:      map[string]bool{"name": true},
		})
	require.Error(t, err)
	assert.Equal(t, "reading policy group after edit: boom", err.Error())
}

func TestPolicyGroupEdit_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		failingPolicyGroupEditFactory(errors.New("not logged in")),
		"prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			rename:       "production",
			changed:      map[string]bool{"name": true},
		})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}

func TestPolicyGroupEdit_InvalidPolicyPackRef(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat:  defaultPolicyGroupGetOutputFormat(),
			addPolicyPack: []string{"@1"},
			changed:       map[string]bool{"add-policy-pack": true},
		})
	require.Error(t, err)
	assert.Equal(t, `policy pack reference "@1" is missing a name`, err.Error())
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_SetsAndRemovesMatchRules(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: apitype.GetPolicyGroupResponse{
		Name: "compliance",
		Mode: apitype.PolicyGroupModeAudit,
	}}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "compliance", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			addStack:     []string{"web/prod"},
			stackRule:    "",
			accountRule:  "env=prod OR team",
			changed: map[string]bool{
				"add-stack": true, "stack-rule": true, "account-rule": true,
			},
		})
	require.NoError(t, err)

	// One GET checks the mode before the account rule is written; one renders the result.
	assert.Equal(t, 2, c.getCalls)
	require.Len(t, c.updates, 1)
	body, err := json.Marshal(c.updates[0])
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"addStack": {"name": "prod", "routingProject": "web"}},
		{"removeStackMatchRule": true},
		{"accountMatchRule": `+binaryJSON("Or",
		tagEqualsJSON(accountCtxJSON, "env", "prod"),
		hasTagJSON(accountCtxJSON, "team"))+`}
	]`, string(body))
}

func TestPolicyGroupEdit_StackRuleAndRemoveAccountRule(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: apitype.GetPolicyGroupResponse{
		Name: "prod-policies",
		Mode: apitype.PolicyGroupModePreventative,
	}}
	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			stackRule:    "env!=dev",
			accountRule:  " ",
			changed:      map[string]bool{"stack-rule": true, "account-rule": true},
		})
	require.NoError(t, err)

	// Removing an account rule does not check the mode before the write.
	assert.Equal(t, 1, c.getCalls)
	require.Len(t, c.updates, 1)
	body, err := json.Marshal(c.updates[0])
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"stackMatchRule": `+notJSON(tagEqualsJSON(stackCtxJSON, "env", "dev"))+`},
		{"removeAccountMatchRule": true}
	]`, string(body))
}

func TestPolicyGroupEdit_AccountRuleOnPreventativeGroup(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: apitype.GetPolicyGroupResponse{
		Name: "prod-policies",
		Mode: apitype.PolicyGroupModePreventative,
	}}
	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			accountRule:  "env=prod",
			changed:      map[string]bool{"account-rule": true},
		})
	require.Error(t, err)
	assert.Equal(t, cmdCmd.ConfigurationError{
		Message: `--account-rule requires an audit policy group; "prod-policies" is preventative`,
	}, err)
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_AccountRuleModeCheckError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getErr: errors.New("not found"), getErrOn: 1}
	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		stubPolicyGroupEditFactory(c, "acme"), "missing", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			accountRule:  "env=prod",
			changed:      map[string]bool{"account-rule": true},
		})
	require.Error(t, err)
	assert.Equal(t, "reading policy group: not found", err.Error())
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_InvalidRule(t *testing.T) {
	t.Parallel()

	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		failingPolicyGroupEditFactory(errors.New("factory must not be called")), "prod-policies",
		policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			stackRule:    `{"__type": "PermissionExpressionStack"}`,
			changed:      map[string]bool{"stack-rule": true},
		})
	require.Error(t, err)
	assert.IsType(t, cmdCmd.ConfigurationError{}, err)
	assert.Equal(t,
		"invalid --stack-rule: invalid JSON expression: type 'PermissionExpressionStack' not recognized",
		err.Error())
}

func TestPolicyGroupEdit_RuleFlagsParse(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: apitype.GetPolicyGroupResponse{
		Name: "prod-policies",
		Mode: apitype.PolicyGroupModeAudit,
	}}
	cmd := newPolicyGroupEditCmdWith(stubPolicyGroupEditFactory(c, "acme"))
	cmd.SetArgs([]string{"prod-policies", "--stack-rule", "env=prod", "--account-rule", ""})
	cmd.SetOut(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	require.Len(t, c.updates, 1)
	body, err := json.Marshal(c.updates[0])
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"stackMatchRule": `+tagEqualsJSON(stackCtxJSON, "env", "prod")+`},
		{"removeAccountMatchRule": true}
	]`, string(body))
}
