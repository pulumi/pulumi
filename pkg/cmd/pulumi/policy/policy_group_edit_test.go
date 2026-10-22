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
)

// mockPolicyGroupEditClient records each BatchUpdatePolicyGroup call and
// returns getResp / getErr from GetPolicyGroup. updateErr forces
// BatchUpdatePolicyGroup to fail.
type mockPolicyGroupEditClient struct {
	updates      [][]cloudapitype.UpdatePolicyGroupRequest
	updateGroups []string
	updateErr    error

	getResp     cloudapitype.PolicyGroup
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
) (cloudapitype.PolicyGroup, error) {
	m.getCalls++
	m.gotGetOrg = org
	m.gotGetGroup = policyGroup
	if m.getErrOn > 0 && m.getCalls == m.getErrOn {
		return cloudapitype.PolicyGroup{}, m.getErr
	}
	if m.getErrOn == 0 && m.getErr != nil {
		return cloudapitype.PolicyGroup{}, m.getErr
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

	resp := cloudapitype.PolicyGroup{
		Name:         "production",
		IsOrgDefault: false,
		EntityType:   cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         cloudapitype.PolicyGroupModeAudit,
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
		{policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{NewName: new("production")})},
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

	c := &mockPolicyGroupEditClient{getResp: cloudapitype.PolicyGroup{
		Name:       "production",
		EntityType: cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:       cloudapitype.PolicyGroupModePreventative,
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
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{NewName: new("production")}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			AddStack: &cloudapitype.AppPulumiStackReference{Name: "prod", RoutingProject: "web"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			AddStack: &cloudapitype.AppPulumiStackReference{Name: "standalone"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			RemoveStack: &cloudapitype.AppPulumiStackReference{Name: "legacy", RoutingProject: "web"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			AddInsightsAccount: &cloudapitype.AppInsightsAccountReference{Name: "acct-2"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			RemoveInsightsAccount: &cloudapitype.AppInsightsAccountReference{Name: "acct-1"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			AddPolicyPack: &cloudapitype.AppPolicyPackMetadata{Name: "aws-guardrails", Version: 3},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			AddPolicyPack: &cloudapitype.AppPolicyPackMetadata{Name: "tagging"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			RemovePolicyPack: &cloudapitype.AppPolicyPackMetadata{Name: "old-pack", VersionTag: "stable"},
		}),
		policyGroupUpdate(cloudapitype.AppUpdatePolicyGroupRequest{
			RemovePolicyPack: &cloudapitype.AppPolicyPackMetadata{Name: "unpinned"},
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
			"--stack-rule, --remove-stack-rule, --account-rule, --remove-account-rule",
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

	c := &mockPolicyGroupEditClient{getResp: cloudapitype.PolicyGroup{
		Name: "compliance",
		Mode: cloudapitype.PolicyGroupModeAudit,
	}}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "compliance", policyGroupEditArgs{
			outputFormat:    defaultPolicyGroupGetOutputFormat(),
			addStack:        []string{"web/prod"},
			removeStackRule: true,
			accountRule:     []string{"env=prod", "team"},
			changed: map[string]bool{
				"add-stack": true, "remove-stack-rule": true, "account-rule": true,
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

	c := &mockPolicyGroupEditClient{getResp: cloudapitype.PolicyGroup{
		Name: "prod-policies",
		Mode: cloudapitype.PolicyGroupModePreventative,
	}}
	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat:      defaultPolicyGroupGetOutputFormat(),
			stackRule:         []string{"env!=dev"},
			removeAccountRule: true,
			changed:           map[string]bool{"stack-rule": true, "remove-account-rule": true},
		})
	require.NoError(t, err)

	// Without --account-rule the mode is not checked before the write.
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

	c := &mockPolicyGroupEditClient{getResp: cloudapitype.PolicyGroup{
		Name: "prod-policies",
		Mode: cloudapitype.PolicyGroupModePreventative,
	}}
	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			accountRule:  []string{"env=prod"},
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
			accountRule:  []string{"env=prod"},
			changed:      map[string]bool{"account-rule": true},
		})
	require.Error(t, err)
	assert.Equal(t, "reading policy group: not found", err.Error())
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_RuleFlagConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args policyGroupEditArgs
		want string
	}{
		{
			args: policyGroupEditArgs{
				stackRule: []string{"env=prod"}, removeStackRule: true,
				changed: map[string]bool{"stack-rule": true, "remove-stack-rule": true},
			},
			want: "--stack-rule and --remove-stack-rule cannot be used together",
		},
		{
			args: policyGroupEditArgs{
				accountRule: []string{"env=prod"}, removeAccountRule: true,
				changed: map[string]bool{"account-rule": true, "remove-account-rule": true},
			},
			want: "--account-rule and --remove-account-rule cannot be used together",
		},
	}
	for _, tt := range tests {
		tt.args.outputFormat = defaultPolicyGroupGetOutputFormat()
		err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
			failingPolicyGroupEditFactory(errors.New("factory must not be called")), "prod-policies", tt.args)
		require.Error(t, err)
		assert.Equal(t, cmdCmd.ConfigurationError{Message: tt.want}, err)
	}
}

func TestPolicyGroupEdit_InvalidRule(t *testing.T) {
	t.Parallel()

	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		failingPolicyGroupEditFactory(errors.New("factory must not be called")), "prod-policies",
		policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			stackRule:    []string{`{"__type": "PermissionExpressionStack"}`},
			changed:      map[string]bool{"stack-rule": true},
		})
	require.Error(t, err)
	assert.IsType(t, cmdCmd.ConfigurationError{}, err)
	assert.Equal(t,
		"invalid --stack-rule: invalid JSON expression: type 'PermissionExpressionStack' not recognized",
		err.Error())
}

func TestPolicyGroupEdit_RemoveRuleFalseIsNoChange(t *testing.T) {
	t.Parallel()

	err := runPolicyGroupEdit(t.Context(), &bytes.Buffer{},
		failingPolicyGroupEditFactory(errors.New("factory must not be called")), "prod-policies",
		policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			changed:      map[string]bool{"remove-stack-rule": true},
		})
	require.ErrorIs(t, err, errNoPolicyGroupChanges)
}

func TestPolicyGroupEdit_RuleFlagsParse(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getResp: cloudapitype.PolicyGroup{
		Name: "prod-policies",
		Mode: cloudapitype.PolicyGroupModeAudit,
	}}
	cmd := newPolicyGroupEditCmdWith(stubPolicyGroupEditFactory(c, "acme"))
	cmd.SetArgs([]string{"prod-policies", "--stack-rule", "env=prod", "--remove-account-rule"})
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
