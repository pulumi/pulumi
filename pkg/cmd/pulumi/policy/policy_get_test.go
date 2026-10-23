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
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOrgPolicyPackName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in        string
		org, name string
		wantErr   bool
	}{
		{in: "acme/pack", org: "acme", name: "pack"},
		{in: "pack", name: "pack"},
		{in: "a/b/c", wantErr: true},
		{in: "acme/", wantErr: true},
		{in: "", wantErr: true},
		{in: "/pack", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			org, name, err := parseOrgPolicyPackName(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.org, org)
			assert.Equal(t, tt.name, name)
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
	// pages holds the packs returned by successive ListPolicyPacks calls.
	pages [][]cloudapitype.AppPolicyPackWithVersions
	pack  cloudapitype.AppGetPolicyPackResponse

	listOrgs   []string
	listTokens []*string
	gotOrg     string
	gotName    string
	gotVersion string

	listErr    error
	getPackErr error
}

func (m *mockPolicyGetClient) ListPolicyPacks(
	_ context.Context, org string, token *string,
) (cloudapitype.AppListPolicyPacksResponse, *string, error) {
	page := len(m.listOrgs)
	m.listOrgs = append(m.listOrgs, org)
	m.listTokens = append(m.listTokens, token)
	if m.listErr != nil {
		return cloudapitype.AppListPolicyPacksResponse{}, nil, m.listErr
	}
	var next *string
	if page+1 < len(m.pages) {
		next = new(fmt.Sprintf("page-%d", page+1))
	}
	return cloudapitype.AppListPolicyPacksResponse{PolicyPacks: m.pages[page]}, next, nil
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

func samplePolicyGetClient() *mockPolicyGetClient {
	return &mockPolicyGetClient{
		pages: [][]cloudapitype.AppPolicyPackWithVersions{{{
			Name: "aws-guardrails", Versions: []int{2, 3}, VersionTags: []string{"1.1.0", "1.2.0"},
		}}},
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
	assert.Equal(t, "1.2.0", got["versionTag"])
	policies := got["policies"].([]any)
	assert.Equal(t, "mandatory", policies[0].(map[string]any)["enforcementLevel"])
	assert.NotContains(t, got, "organization")
	assert.NotContains(t, got, "policyGroups")
	assert.Equal(t, "1.2.0", c.gotVersion)
}

func TestPolicyGet_ExplicitOrgAndVersion(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	executePolicyGet(t, c, "other-org/aws-guardrails", "1.1.0", "--output", "json")
	assert.Equal(t, "other-org", c.gotOrg)
	assert.Equal(t, "1.1.0", c.gotVersion)
	assert.Empty(t, c.listOrgs, "an explicit version must not list policy packs")
}

func TestPolicyGet_LatestListsPacksInResolvedOrg(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	executePolicyGet(t, c, "other-org/aws-guardrails", "latest", "--output", "json")
	assert.Equal(t, []string{"other-org"}, c.listOrgs)
	assert.Equal(t, "other-org", c.gotOrg)
	assert.Equal(t, "1.2.0", c.gotVersion)
}

func TestPolicyGet_LatestFollowsContinuationTokens(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.pages = [][]cloudapitype.AppPolicyPackWithVersions{
		{{Name: "other-pack", Versions: []int{1}, VersionTags: []string{"0.1.0"}}},
		{{Name: "aws-guardrails", Versions: []int{5}, VersionTags: []string{"2.0.0"}}},
	}
	executePolicyGet(t, c, "aws-guardrails", "--output", "json")
	require.Len(t, c.listTokens, 2)
	assert.Nil(t, c.listTokens[0])
	assert.Equal(t, "page-1", *c.listTokens[1])
	assert.Equal(t, "2.0.0", c.gotVersion)
}

func TestPolicyGet_PackNotFound(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	_, err := executePolicyGetErr(t, c, "missing", "--output", "json")
	require.ErrorContains(t, err, `policy pack "missing" not found in organization "acme"`)
	assert.Empty(t, c.gotName, "GetPolicyPack must not be called")
}

func TestPolicyGet_ListErrorPropagates(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.listErr = errors.New("boom")
	_, err := executePolicyGetErr(t, c, "aws-guardrails")
	require.ErrorContains(t, err, "boom")
	assert.Empty(t, c.gotName, "GetPolicyPack must not be called")
}

func TestPolicyGet_TooManyArgs(t *testing.T) {
	t.Parallel()

	_, err := executePolicyGetErr(t, samplePolicyGetClient(), "acme/aws-guardrails", "1.2.0", "extra")
	require.Error(t, err)
}

func TestPolicyGet_NoPolicies_JSONEmptyArray(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.pack.Policies = nil
	out := executePolicyGet(t, c, "aws-guardrails", "--output", "json")
	assert.Contains(t, out, `"policies": []`)
}

func TestPolicyGet_Text(t *testing.T) {
	t.Parallel()

	out := executePolicyGet(t, samplePolicyGetClient(), "aws-guardrails")
	assert.Contains(t, out, "POLICIES")
	assert.Contains(t, out, "s3-no-public")
	assert.Contains(t, out, "mandatory")
	assert.Contains(t, out, "1.2.0 (v3)")
}

func TestPolicyGet_TextUntaggedVersion(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.pack.VersionTag = ""
	out := executePolicyGet(t, c, "aws-guardrails", "3")
	assert.Contains(t, out, "v3")
	assert.NotContains(t, out, "(3)")
}

func TestPolicyGet_GetPolicyPackErrorPropagates(t *testing.T) {
	t.Parallel()

	c := samplePolicyGetClient()
	c.getPackErr = errors.New("boom")
	out, err := executePolicyGetErr(t, c, "aws-guardrails", "--output", "json")
	require.ErrorContains(t, err, "boom")
	assert.NotContains(t, out, `"policies"`)
}
