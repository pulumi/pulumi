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

// mockPolicyGroupEditClient records the sequence of UpdatePolicyGroup calls,
// returns getResp / getErr from GetPolicyGroup, and can inject an error on the
// Nth (1-indexed) UpdatePolicyGroup call via updateErrOn.
type mockPolicyGroupEditClient struct {
	updates      []apitype.UpdatePolicyGroupRequest
	updateGroups []string
	updateErrOn  int
	updateErr    error
	getResp      apitype.GetPolicyGroupResponse
	getErr       error
	gotGetOrg    string
	gotGetGroup  string
}

func (m *mockPolicyGroupEditClient) UpdatePolicyGroup(
	_ context.Context, _, policyGroup string, req apitype.UpdatePolicyGroupRequest,
) error {
	m.updates = append(m.updates, req)
	m.updateGroups = append(m.updateGroups, policyGroup)
	if m.updateErrOn > 0 && len(m.updates) == m.updateErrOn {
		return m.updateErr
	}
	return nil
}

func (m *mockPolicyGroupEditClient) GetPolicyGroup(
	_ context.Context, org, policyGroup string,
) (apitype.GetPolicyGroupResponse, error) {
	m.gotGetOrg = org
	m.gotGetGroup = policyGroup
	if m.getErr != nil {
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

func ptrString(s string) *string { return &s }

func TestPolicyGroupEdit_RenameOnly_DefaultOutput(t *testing.T) {
	t.Parallel()

	resp := apitype.GetPolicyGroupResponse{
		Name:         "production",
		IsOrgDefault: false,
		EntityType:   apitype.Stacks,
		Mode:         apitype.PolicyGroupModeAudit,
	}
	c := &mockPolicyGroupEditClient{getResp: resp}

	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			newName:      "production",
			changed:      map[string]bool{"new-name": true},
		})
	require.NoError(t, err)

	assert.Equal(t, []apitype.UpdatePolicyGroupRequest{
		{NewName: ptrString("production")},
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

	resp := apitype.GetPolicyGroupResponse{
		Name:         "production",
		IsOrgDefault: false,
		EntityType:   apitype.Stacks,
		Mode:         apitype.PolicyGroupModePreventative,
		Stacks: []apitype.PulumiStackReference{
			{Name: "prod", RoutingProject: "web"},
		},
		AppliedPolicyPacks: []apitype.PolicyPackMetadata{
			{Name: "aws-guardrails", Version: 3},
		},
		Accounts: []string{"acct-2"},
	}
	c := &mockPolicyGroupEditClient{getResp: resp}

	args := policyGroupEditArgs{
		outputFormat:          defaultPolicyGroupGetOutputFormat(),
		newName:               "production",
		addStack:              []string{"web/prod", "standalone"},
		removeStack:           []string{"web/legacy"},
		addPolicyPack:         []string{"aws-guardrails@3", "tagging"},
		removePolicyPack:      []string{"old-pack@stable"},
		addInsightsAccount:    []string{"acct-2"},
		removeInsightsAccount: []string{"acct-1"},
		changed: map[string]bool{
			"new-name": true, "add-stack": true, "remove-stack": true,
			"add-policy-pack": true, "remove-policy-pack": true,
			"add-insights-account": true, "remove-insights-account": true,
		},
	}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", args)
	require.NoError(t, err)

	assert.Equal(t, []apitype.UpdatePolicyGroupRequest{
		{NewName: ptrString("production")},
		{AddStack: &apitype.PulumiStackReference{Name: "prod", RoutingProject: "web"}},
		{AddStack: &apitype.PulumiStackReference{Name: "standalone"}},
		{AddPolicyPack: &apitype.PolicyPackMetadata{Name: "aws-guardrails", Version: 3}},
		{AddPolicyPack: &apitype.PolicyPackMetadata{Name: "tagging"}},
		{AddInsightsAccount: ptrString("acct-2")},
		{RemoveStack: &apitype.PulumiStackReference{Name: "legacy", RoutingProject: "web"}},
		{RemovePolicyPack: &apitype.PolicyPackMetadata{Name: "old-pack", VersionTag: "stable"}},
		{RemoveInsightsAccount: ptrString("acct-1")},
	}, c.updates)

	assert.Equal(t, []string{
		"prod-policies",
		"production", "production",
		"production", "production",
		"production",
		"production",
		"production",
		"production",
	}, c.updateGroups)
	assert.Equal(t, "production", c.gotGetGroup)

	assert.JSONEq(t, `{
		"name": "production",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "preventative",
		"stacks": [{"name": "prod", "routingProject": "web"}],
		"appliedPolicyPacks": [
			{"name": "aws-guardrails", "displayName": "", "version": 3, "versionTag": ""}
		],
		"accounts": ["acct-2"]
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
		"no changes specified; pass at least one of --new-name, --add-stack, --remove-stack, "+
			"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account",
		err.Error())
	assert.Empty(t, c.updates)
}

func TestPolicyGroupEdit_UpdateErrorStopsSubsequentPatches(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{
		updateErrOn: 2,
		updateErr:   errors.New("conflict"),
	}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			addStack:     []string{"web/prod", "web/staging"},
			changed:      map[string]bool{"add-stack": true},
		})
	require.Error(t, err)
	assert.Equal(t, "conflict", err.Error())

	// Two PATCHes were attempted; the third never ran and Get was skipped.
	assert.Equal(t, []apitype.UpdatePolicyGroupRequest{
		{AddStack: &apitype.PulumiStackReference{Name: "prod", RoutingProject: "web"}},
		{AddStack: &apitype.PulumiStackReference{Name: "staging", RoutingProject: "web"}},
	}, c.updates)
	assert.Empty(t, c.gotGetGroup)
}

func TestPolicyGroupEdit_GetAfterEditError(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupEditClient{getErr: errors.New("boom")}
	var buf bytes.Buffer
	err := runPolicyGroupEdit(t.Context(), &buf,
		stubPolicyGroupEditFactory(c, "acme"), "prod-policies", policyGroupEditArgs{
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			newName:      "production",
			changed:      map[string]bool{"new-name": true},
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
			newName:      "production",
			changed:      map[string]bool{"new-name": true},
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
