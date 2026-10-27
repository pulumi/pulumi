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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockPolicyGroupEditClient records each BatchUpdatePolicyGroup call and
// returns getResp / getErr from GetPolicyGroup. updateErr forces
// BatchUpdatePolicyGroup to fail.
type mockPolicyGroupEditClient struct {
	updates      [][]apitype.UpdatePolicyGroupRequest
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
	_ context.Context, _, policyGroup string, reqs []apitype.UpdatePolicyGroupRequest,
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
		EntityType:   apitype.Stacks,
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
	assert.Equal(t, [][]apitype.UpdatePolicyGroupRequest{
		{{NewName: new("production")}},
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

	c := &mockPolicyGroupEditClient{getResp: apitype.GetPolicyGroupResponse{
		Name:       "production",
		EntityType: apitype.Stacks,
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

	assert.Equal(t, [][]apitype.UpdatePolicyGroupRequest{{
		{NewName: new("production")},
		{AddStack: &apitype.PulumiStackReference{Name: "prod", RoutingProject: "web"}},
		{AddStack: &apitype.PulumiStackReference{Name: "standalone"}},
		{RemoveStack: &apitype.PulumiStackReference{Name: "legacy", RoutingProject: "web"}},
		{AddInsightsAccount: &apitype.InsightsAccountReference{Name: "acct-2"}},
		{RemoveInsightsAccount: &apitype.InsightsAccountReference{Name: "acct-1"}},
		{AddPolicyPack: &apitype.PolicyPackMetadata{Name: "aws-guardrails", Version: 3}},
		{AddPolicyPack: &apitype.PolicyPackMetadata{Name: "tagging"}},
		{RemovePolicyPack: &apitype.PolicyPackMetadata{Name: "old-pack", VersionTag: "stable"}},
		{RemovePolicyPack: &apitype.PolicyPackMetadata{Name: "unpinned"}},
	}}, c.updates)

	// The batch addresses the original group name; the post-edit GET uses
	// the new name.
	assert.Equal(t, []string{"prod-policies"}, c.updateGroups)
	assert.Equal(t, 1, c.getCalls)
	assert.Equal(t, "production", c.gotGetGroup)
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
			"--add-policy-pack, --remove-policy-pack, --add-insights-account, --remove-insights-account",
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
