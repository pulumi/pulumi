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
	"io"
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

	// beforeResp, when set, is returned by the first GetPolicyGroup call.
	beforeResp *cloudapitype.PolicyGroup
	packs      []cloudapitype.AppPolicyPackWithVersions
	policies   map[string][]cloudapitype.AppPolicy
	packGets   []string
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
	if m.beforeResp != nil && m.getCalls == 1 {
		return *m.beforeResp, nil
	}
	return m.getResp, nil
}

func (m *mockPolicyGroupEditClient) ListPolicyPacks(
	context.Context, string, *string,
) (cloudapitype.AppListPolicyPacksResponse, *string, error) {
	return cloudapitype.AppListPolicyPacksResponse{PolicyPacks: m.packs}, nil, nil
}

func (m *mockPolicyGroupEditClient) GetPolicyPack(
	_ context.Context, _, name, versionTag string,
) (cloudapitype.AppGetPolicyPackResponse, error) {
	m.packGets = append(m.packGets, name+"@"+versionTag)
	return cloudapitype.AppGetPolicyPackResponse{
		Name: name, VersionTag: versionTag, Policies: m.policies[versionTag],
	}, nil
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
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
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
Entity type:           stacks
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
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
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
	assert.Equal(t, 2, c.getCalls)
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
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			changed:      map[string]bool{},
		})
	require.Error(t, err)
	assert.Equal(t,
		"no changes specified; pass at least one of --name, --add-stack, --remove-stack, "+
			"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account, "+
			"--agent-pool-id",
		err.Error())
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_UpdateError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{updateErr: errors.New("conflict")}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
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
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
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
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
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
	err := runPolicyGroupEdit(t.Context(), &buf, io.Discard,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat:  defaultPolicyGroupGetOutputFormat(),
			addPolicyPack: []string{"@1"},
			changed:       map[string]bool{"add-policy-pack": true},
		})
	require.Error(t, err)
	assert.Equal(t, `policy pack reference "@1" is missing a name`, err.Error())
	assert.Empty(t, c.updates)
}

func updatesJSON(t *testing.T, updates []cloudapitype.UpdatePolicyGroupRequest) string {
	t.Helper()
	b, err := json.Marshal(updates)
	require.NoError(t, err)
	return string(b)
}

func TestPolicyGroupEdit_AddPolicyPackKeepsExistingSettings(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{
		beforeResp: &cloudapitype.PolicyGroup{AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{{
			Name: "aws-guardrails", VersionTag: "1.0.0",
			Config: map[string]*json.RawMessage{
				"all": new(json.RawMessage(`{"enforcementLevel":"mandatory"}`)),
				"s3":  new(json.RawMessage(`{"limit":3}`)),
			},
			Environments: []string{"policies/creds"},
		}}},
	}
	var stderr bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), io.Discard, &stderr,
		stubPolicyGroupEditFactory(c, "acme"), "prod", policyGroupEditArgs{
			outputFormat:  defaultPolicyGroupGetOutputFormat(),
			addPolicyPack: []string{"aws-guardrails@1.0.0"},
			changed:       map[string]bool{"add-policy-pack": true},
		})
	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Empty(t, c.packGets)

	require.Len(t, c.updates, 1)
	assert.JSONEq(t, `[{"addPolicyPack": {
		"name": "aws-guardrails", "displayName": "", "version": 0, "versionTag": "1.0.0",
		"config": {"all": {"enforcementLevel": "mandatory"}, "s3": {"limit": 3}},
		"environments": ["policies/creds"]
	}}]`, updatesJSON(t, c.updates[0]))
}

func TestPolicyGroupEdit_AddPolicyPackWithSettings(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{
		beforeResp: &cloudapitype.PolicyGroup{AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{{
			Name: "aws-guardrails", VersionTag: "1.0.0",
			Config: map[string]*json.RawMessage{
				"s3":   new(json.RawMessage(`{"enforcementLevel":"advisory","limit":3}`)),
				"gone": new(json.RawMessage(`{"limit":1}`)),
			},
			Environments: []string{"policies/creds@1"},
		}}},
		policies: map[string][]cloudapitype.AppPolicy{"2.0.0": policiesNamed("s3", "iam")},
	}
	var stderr bytes.Buffer
	args := policyGroupEditArgs{
		outputFormat:  defaultPolicyGroupGetOutputFormat(),
		addPolicyPack: []string{"aws-guardrails@2.0.0"},
		packSettings: packSettingsFlags{
			enforcementLevel:   "remediate",
			policyEnforcement:  []string{"iam=advisory"},
			environments:       []string{"policies/creds@2"},
			removeEnvironments: []string{"policies/other"},
		},
		changed: map[string]bool{"add-policy-pack": true},
	}
	err := runPolicyGroupEdit(t.Context(), io.Discard, &stderr, stubPolicyGroupEditFactory(c, "acme"), "prod", args)
	require.NoError(t, err)

	assert.Equal(t, remediateWarning+
		"warning: removed configuration for policies not in version 2.0.0 of policy pack \"aws-guardrails\": gone\n",
		stderr.String())
	assert.Equal(t, []string{"aws-guardrails@2.0.0"}, c.packGets)
	require.Len(t, c.updates, 1)
	assert.JSONEq(t, `[{"addPolicyPack": {
		"name": "aws-guardrails", "displayName": "", "version": 0, "versionTag": "2.0.0",
		"config": {
			"all": {"enforcementLevel": "remediate"},
			"s3": {"limit": 3},
			"iam": {"enforcementLevel": "advisory"}
		},
		"environments": ["policies/creds@2"]
	}}]`, updatesJSON(t, c.updates[0]))
}

func TestPolicyGroupEdit_PackSettingsRequireOneAddPolicyPack(t *testing.T) {
	t.Parallel()

	for _, packs := range [][]string{nil, {"a", "b"}} {
		c := &mockPolicyGroupEditClient{}
		err := runPolicyGroupEdit(t.Context(), io.Discard, io.Discard,
			stubPolicyGroupEditFactory(c, "acme"), "prod", policyGroupEditArgs{
				outputFormat:  defaultPolicyGroupGetOutputFormat(),
				addPolicyPack: packs,
				packSettings:  packSettingsFlags{enforcementLevel: "mandatory"},
				changed:       map[string]bool{"add-policy-pack": len(packs) > 0},
			})
		var cfgErr cmdCmd.ConfigurationError
		require.ErrorAs(t, err, &cfgErr)
		assert.Contains(t, err.Error(), "pass exactly one --add-policy-pack")
		assert.Empty(t, c.updates)
		assert.Zero(t, c.getCalls)
	}
}

func TestPolicyGroupEdit_InvalidEnforcementLevel(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{}
	err := runPolicyGroupEdit(t.Context(), io.Discard, io.Discard,
		stubPolicyGroupEditFactory(c, "acme"), "prod", policyGroupEditArgs{
			outputFormat:  defaultPolicyGroupGetOutputFormat(),
			addPolicyPack: []string{"a"},
			packSettings:  packSettingsFlags{enforcementLevel: "strict"},
			changed:       map[string]bool{"add-policy-pack": true},
		})
	var cfgErr cmdCmd.ConfigurationError
	require.ErrorAs(t, err, &cfgErr)
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_AgentPoolID(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ value, want string }{
		{"pool-123", `[{"agentPoolId": "pool-123"}]`},
		{"", `[{"agentPoolId": ""}]`},
	} {
		c := &mockPolicyGroupEditClient{}
		err := runPolicyGroupEdit(t.Context(), io.Discard, io.Discard,
			stubPolicyGroupEditFactory(c, "acme"), "prod", policyGroupEditArgs{
				outputFormat: defaultPolicyGroupGetOutputFormat(),
				agentPoolID:  tt.value,
				changed:      map[string]bool{"agent-pool-id": true},
			})
		require.NoError(t, err)
		require.Len(t, c.updates, 1)
		assert.JSONEq(t, tt.want, updatesJSON(t, c.updates[0]))
		assert.Equal(t, 1, c.getCalls, "no read before the edit without --add-policy-pack")
	}
}

func TestPolicyGroupEdit_HelpListsNewFlags(t *testing.T) {
	t.Parallel()

	cmd := newPolicyGroupEditCmdWith(stubPolicyGroupEditFactory(&mockPolicyGroupEditClient{}, "acme"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
	help := out.String()
	for _, flag := range []string{
		"--agent-pool-id", "--enforcement-level", "--policy-enforcement", "--environment", "--remove-environment",
	} {
		assert.Contains(t, help, flag)
	}
	assert.Contains(t, help, "advisory, mandatory, remediate, disabled")
}
