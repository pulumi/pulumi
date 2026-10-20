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

func TestParsePolicyPackVersionRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in                 string
		org, name, version string
		wantErr            bool
	}{
		{in: "acme/pack@1.2.0", org: "acme", name: "pack", version: "1.2.0"},
		{in: "pack", name: "pack"},
		{in: "acme/pack@latest", org: "acme", name: "pack"},
		{in: "pack@latest", name: "pack"},
		{in: "a/b/c", wantErr: true},
		{in: "acme/", wantErr: true},
		{in: "acme/pack@", wantErr: true},
		{in: "", wantErr: true},
		{in: "/pack", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			org, name, version, err := parsePolicyPackVersionRef(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.org, org)
			assert.Equal(t, tt.name, name)
			assert.Equal(t, tt.version, version)
		})
	}
}

func TestResolveLatestVersion_PicksHighestNumber(t *testing.T) {
	t.Parallel()

	packs := []cloudapitype.AppPolicyPackWithVersions{{
		Name:        "pack",
		Versions:    []int{1, 3, 2},
		VersionTags: []string{"1.0.0", "1.2.0", "1.1.0"},
	}}
	tag, err := resolveLatestVersionTag(packs, "acme", "pack")
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", tag)
}

func TestResolveLatestVersion_EmptyTagFallsBackToNumber(t *testing.T) {
	t.Parallel()

	packs := []cloudapitype.AppPolicyPackWithVersions{{
		Name:        "pack",
		Versions:    []int{4},
		VersionTags: []string{""},
	}}
	tag, err := resolveLatestVersionTag(packs, "acme", "pack")
	require.NoError(t, err)
	assert.Equal(t, "4", tag)
}

func TestResolveLatestVersion_NotFound(t *testing.T) {
	t.Parallel()

	_, err := resolveLatestVersionTag(nil, "acme", "missing")
	require.ErrorContains(t, err, `policy pack "missing" not found in organization "acme"`)
}

type mockPolicyGetClient struct {
	packs     []cloudapitype.AppPolicyPackWithVersions
	pack      cloudapitype.AppGetPolicyPackResponse
	summaries []cloudapitype.AppPolicyGroupSummary
	groups    map[string]cloudapitype.PolicyGroup

	gotOrg       string
	gotName      string
	gotVersion   string
	groupsLoaded []string

	getPackErr  error
	getGroupErr error
}

func (m *mockPolicyGetClient) ListPolicyPacks(
	_ context.Context, _ string, _ *string,
) (cloudapitype.AppListPolicyPacksResponse, *string, error) {
	return cloudapitype.AppListPolicyPacksResponse{PolicyPacks: m.packs}, nil, nil
}

func (m *mockPolicyGetClient) GetPolicyPack(
	_ context.Context, org, name, versionTag string,
) (cloudapitype.AppGetPolicyPackResponse, error) {
	m.gotOrg, m.gotName, m.gotVersion = org, name, versionTag
	if m.getPackErr != nil {
		return cloudapitype.AppGetPolicyPackResponse{}, m.getPackErr
	}
	return m.pack, nil
}

func (m *mockPolicyGetClient) ListPolicyGroups(
	_ context.Context, _ string, _ *string,
) (cloudapitype.AppListPolicyGroupsResponse, *string, error) {
	return cloudapitype.AppListPolicyGroupsResponse{PolicyGroups: m.summaries}, nil, nil
}

func (m *mockPolicyGetClient) GetPolicyGroup(
	_ context.Context, _, group string,
) (cloudapitype.PolicyGroup, error) {
	m.groupsLoaded = append(m.groupsLoaded, group)
	if m.getGroupErr != nil {
		return cloudapitype.PolicyGroup{}, m.getGroupErr
	}
	return m.groups[group], nil
}

func samplePolicyGetClient() *mockPolicyGetClient {
	return &mockPolicyGetClient{
		packs: []cloudapitype.AppPolicyPackWithVersions{{
			Name: "aws-guardrails", Versions: []int{2, 3}, VersionTags: []string{"1.1.0", "1.2.0"},
		}},
		pack: cloudapitype.AppGetPolicyPackResponse{
			Name:        "aws-guardrails",
			DisplayName: "AWS Guardrails",
			Version:     3,
			VersionTag:  "1.2.0",
			Policies: []cloudapitype.AppPolicy{{
				Name:             "s3-no-public",
				Description:      "S3 buckets must not be public",
				EnforcementLevel: cloudapitype.AppEnforcementLevelMandatory,
			}},
		},
		summaries: []cloudapitype.AppPolicyGroupSummary{
			{Name: "prod", NumEnabledPolicyPacks: 1},
			{Name: "dev", NumEnabledPolicyPacks: 1},
			{Name: "empty", NumEnabledPolicyPacks: 0},
		},
		groups: map[string]cloudapitype.PolicyGroup{
			"prod": {
				Name: "prod", Mode: cloudapitype.PolicyGroupModePreventative,
				AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{
					{Name: "aws-guardrails", Version: 3, VersionTag: "1.2.0"},
				},
			},
			"dev": {
				Name: "dev", Mode: cloudapitype.PolicyGroupModeAudit,
				AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{
					{Name: "other", Version: 1, VersionTag: "1.0.0"},
				},
			},
		},
	}
}

func executePolicyGet(t *testing.T, c *mockPolicyGetClient, args ...string) string {
	t.Helper()
	out, err := executePolicyGetErr(t, c, args...)
	require.NoError(t, err)
	return out
}

func executePolicyGetErr(t *testing.T, c *mockPolicyGetClient, args ...string) (string, error) {
	t.Helper()
	factory := func(_ context.Context, org string) (policyGetClient, string, error) {
		if org == "" {
			org = "acme"
		}
		return c, org, nil
	}
	cmd := newPolicyGetCmdWith(factory)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return buf.String(), err
}

func TestPolicyGet_JSON(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	out := executePolicyGet(t, c, "aws-guardrails", "--output", "json")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "acme", got["organization"])
	assert.Equal(t, "1.2.0", got["versionTag"])
	policies := got["policies"].([]any)
	assert.Equal(t, "mandatory", policies[0].(map[string]any)["enforcementLevel"])

	groups := got["policyGroups"].([]any)
	require.Len(t, groups, 1)
	prod := groups[0].(map[string]any)
	assert.Equal(t, "prod", prod["name"])
	assert.Equal(t, "1.2.0", prod["versionTag"])
	assert.NotContains(t, c.groupsLoaded, "empty")
	assert.Equal(t, "1.2.0", c.gotVersion)
}

func TestPolicyGet_ExplicitOrgAndVersion(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	executePolicyGet(t, c, "other-org/aws-guardrails@1.1.0", "--output", "json")
	assert.Equal(t, "other-org", c.gotOrg)
	assert.Equal(t, "1.1.0", c.gotVersion)
}

func TestPolicyGet_NotEnabledAnywhere_JSONEmptyArray(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.summaries = nil
	c.pack.Policies = nil
	out := executePolicyGet(t, c, "aws-guardrails", "--output", "json")
	assert.Contains(t, out, `"policyGroups": []`)
	assert.Contains(t, out, `"policies": []`)
}

func TestPolicyGet_Text(t *testing.T) {
	t.Parallel()

	out := executePolicyGet(t, samplePolicyGetClient(), "aws-guardrails")
	assert.Contains(t, out, "POLICIES")
	assert.Contains(t, out, "ENABLED IN POLICY GROUPS")
	assert.Contains(t, out, "s3-no-public")
	assert.Contains(t, out, "mandatory")
	assert.Contains(t, out, "prod")
}

func TestPolicyGet_Text_NotEnabled(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.summaries = nil
	out := executePolicyGet(t, c, "aws-guardrails")
	assert.Contains(t, out, "Not enabled in any policy group.")
}

func TestPolicyGet_GetPolicyPackErrorPropagates(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.getPackErr = errors.New("boom")
	out, err := executePolicyGetErr(t, c, "aws-guardrails", "--output", "json")
	require.ErrorContains(t, err, "boom")
	assert.NotContains(t, out, `"organization"`)
}

func TestPolicyGet_GetPolicyGroupErrorPropagates(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.getGroupErr = errors.New("group boom")
	out, err := executePolicyGetErr(t, c, "aws-guardrails", "--output", "json")
	require.ErrorContains(t, err, "group boom")
	assert.NotContains(t, out, `"organization"`)
}
