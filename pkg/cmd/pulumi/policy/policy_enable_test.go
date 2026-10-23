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
	"os"
	"path/filepath"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type stubPolicyPackRef struct{ org, name string }

func (r stubPolicyPackRef) String() string     { return r.org + "/" + r.name }
func (r stubPolicyPackRef) OrgName() string    { return r.org }
func (r stubPolicyPackRef) Name() tokens.QName { return tokens.QName(r.name) }

type stubPolicyGroupClient struct {
	get func(ctx context.Context, org, group string) (cloudapitype.PolicyGroup, error)
}

func (s stubPolicyGroupClient) GetPolicyGroup(
	ctx context.Context, org, group string,
) (cloudapitype.PolicyGroup, error) {
	return s.get(ctx, org, group)
}

func stubPolicyPackDeps(pack backend.PolicyPack, client policyGroupGetClient) policyPackCmdDeps {
	return func(context.Context, string) (backend.PolicyPack, policyGroupGetClient, error) {
		return pack, client, nil
	}
}

func runPolicyPackCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func runPolicyPackCmdSplit(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SilenceUsage = true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), err
}

// fakePolicyEnableClient returns groups[i] from the i-th GetPolicyGroup call (repeating the
// last one), the latest-version listing from packs, and policies by version tag.
type fakePolicyEnableClient struct {
	groups    []cloudapitype.PolicyGroup
	groupErrs []error
	groupGets []string
	packs     []cloudapitype.AppPolicyPackWithVersions
	listCalls int
	policies  map[string][]cloudapitype.AppPolicy
	packGets  []string
}

func (f *fakePolicyEnableClient) GetPolicyGroup(
	_ context.Context, _, group string,
) (cloudapitype.PolicyGroup, error) {
	i := len(f.groupGets)
	f.groupGets = append(f.groupGets, group)
	if i < len(f.groupErrs) && f.groupErrs[i] != nil {
		return cloudapitype.PolicyGroup{}, f.groupErrs[i]
	}
	if len(f.groups) == 0 {
		return cloudapitype.PolicyGroup{}, nil
	}
	return f.groups[min(i, len(f.groups)-1)], nil
}

func (f *fakePolicyEnableClient) ListPolicyPacks(
	context.Context, string, *string,
) (cloudapitype.AppListPolicyPacksResponse, *string, error) {
	f.listCalls++
	return cloudapitype.AppListPolicyPacksResponse{PolicyPacks: f.packs}, nil, nil
}

func (f *fakePolicyEnableClient) GetPolicyPack(
	_ context.Context, _, name, versionTag string,
) (cloudapitype.AppGetPolicyPackResponse, error) {
	f.packGets = append(f.packGets, versionTag)
	policies, ok := f.policies[versionTag]
	if !ok {
		return cloudapitype.AppGetPolicyPackResponse{}, fmt.Errorf("no version %s", versionTag)
	}
	return cloudapitype.AppGetPolicyPackResponse{Name: name, VersionTag: versionTag, Policies: policies}, nil
}

func stubPolicyEnableDeps(pack backend.PolicyPack, client policyEnableClient) policyEnableDeps {
	return func(context.Context, string) (backend.PolicyPack, policyEnableClient, error) {
		return pack, client, nil
	}
}

// recordingPolicyPack returns a mock pack named acme/<name> that records Enable calls.
func recordingPolicyPack(name string, ops *[]backend.PolicyPackOperation) *backend.MockPolicyPack {
	return &backend.MockPolicyPack{
		RefF: func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", name} },
		EnableF: func(_ context.Context, _ string, op backend.PolicyPackOperation) error {
			*ops = append(*ops, op)
			return nil
		},
	}
}

func runPolicyEnable(
	t *testing.T, pack backend.PolicyPack, client policyEnableClient, args ...string,
) (string, string, error) {
	t.Helper()
	cmd := newPolicyEnableCmdWith(stubPolicyEnableDeps(pack, client))
	var stdout, stderr bytes.Buffer
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}

func rawConfig(t *testing.T, s string) map[string]*json.RawMessage {
	t.Helper()
	var m map[string]*json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(s), &m))
	return m
}

func configJSON(t *testing.T, m map[string]*json.RawMessage) string {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}

func groupWith(packs ...cloudapitype.AppPolicyPackMetadata) cloudapitype.PolicyGroup {
	return cloudapitype.PolicyGroup{AppliedPolicyPacks: packs}
}

func policiesNamed(names ...string) []cloudapitype.AppPolicy {
	policies := make([]cloudapitype.AppPolicy, 0, len(names))
	for _, n := range names {
		policies = append(policies, cloudapitype.AppPolicy{Name: n, EnforcementLevel: "advisory"})
	}
	return policies
}

func TestPolicyEnable_JSONReadsBackGroup(t *testing.T) {
	t.Parallel()

	var enabledGroup string
	pack := &backend.MockPolicyPack{
		RefF: func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
		EnableF: func(_ context.Context, group string, _ backend.PolicyPackOperation) error {
			enabledGroup = group
			return nil
		},
	}
	raw := json.RawMessage(`"mandatory"`)
	group := groupWith(
		cloudapitype.AppPolicyPackMetadata{Name: "other", VersionTag: "9.9.9"},
		cloudapitype.AppPolicyPackMetadata{
			Name: "aws-guardrails", Version: 3, VersionTag: "1.2.0",
			Config: map[string]*json.RawMessage{"all": &raw},
		},
	)
	group.Name = "default-policy-group"
	client := &fakePolicyEnableClient{groups: []cloudapitype.PolicyGroup{group}}

	out, _, err := runPolicyEnable(t, pack, client, "acme/aws-guardrails", "1.2.0", "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "", enabledGroup)
	assert.Equal(t, []string{"default-policy-group", "default-policy-group"}, client.groupGets)

	var got cloudapitype.PolicyGroup
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "default-policy-group", got.Name)
	require.Len(t, got.AppliedPolicyPacks, 2)
	assert.Equal(t, "aws-guardrails", got.AppliedPolicyPacks[1].Name)
	assert.Equal(t, "1.2.0", got.AppliedPolicyPacks[1].VersionTag)
	assert.JSONEq(t, `"mandatory"`, string(*got.AppliedPolicyPacks[1].Config["all"]))
}

func TestPolicyEnable_ExplicitGroup(t *testing.T) {
	t.Parallel()

	var ops []backend.PolicyPackOperation
	client := &fakePolicyEnableClient{
		groups: []cloudapitype.PolicyGroup{
			{},
			{Name: "prod", AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{{Name: "p", VersionTag: "2.0.0"}}},
		},
	}
	out, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client,
		"acme/p", "latest", "--policy-group", "prod", "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, []string{"prod", "prod"}, client.groupGets)
	assert.Contains(t, out, `"name": "prod"`)
	require.Len(t, ops, 1)
	assert.Nil(t, ops[0].VersionTag)
	assert.Zero(t, client.listCalls, "latest needs no resolution when the pack is not yet enabled")
}

func TestPolicyEnable_ReadBackFailure(t *testing.T) {
	t.Parallel()

	var ops []backend.PolicyPackOperation
	client := &fakePolicyEnableClient{groupErrs: []error{nil, errors.New("boom")}}
	out, _, err := runPolicyEnable(t, recordingPolicyPack("aws-guardrails", &ops), client,
		"acme/aws-guardrails", "latest", "--output", "json")
	require.EqualError(t, err, `reading policy group "default-policy-group" after update: boom`)
	require.Len(t, ops, 1)
	assert.Empty(t, out)
}

func TestPolicyEnable_DefaultRendersGroup(t *testing.T) {
	t.Parallel()

	group := cloudapitype.PolicyGroup{
		Name:               "prod",
		AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{{Name: "p", Version: 3, VersionTag: "1.2.0"}},
	}
	var ops []backend.PolicyPackOperation
	client := &fakePolicyEnableClient{groups: []cloudapitype.PolicyGroup{group}}
	out, stderr, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client,
		"acme/p", "latest", "--policy-group", "prod")
	require.NoError(t, err)
	assert.Empty(t, stderr)

	var want bytes.Buffer
	require.NoError(t, renderPolicyGroupGetText(&want, group))
	assert.Equal(t, want.String(), out)
	assert.Equal(t, []string{"prod", "prod"}, client.groupGets)
}

func TestPolicyEnable_ReadBeforeFailureDoesNotWrite(t *testing.T) {
	t.Parallel()

	var ops []backend.PolicyPackOperation
	client := &fakePolicyEnableClient{groupErrs: []error{errors.New("boom")}}
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p", "latest")
	require.EqualError(t, err, `reading policy group "default-policy-group": boom`)
	assert.Empty(t, ops)
}

func TestPolicyEnable_NoFlagsPreservesConfigAndEnvironments(t *testing.T) {
	t.Parallel()

	config := rawConfig(t, `{"all":{"enforcementLevel":"mandatory"},"s3":{"enforcementLevel":"advisory","limit":3}}`)
	client := &fakePolicyEnableClient{groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
		Name: "p", Version: 4, VersionTag: "1.2.0", Config: config, Environments: []string{"policies/creds"},
	})}}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p")
	require.NoError(t, err)

	require.Len(t, ops, 1)
	require.NotNil(t, ops[0].VersionTag)
	assert.Equal(t, "1.2.0", *ops[0].VersionTag)
	assert.JSONEq(t, configJSON(t, config), configJSON(t, ops[0].Config))
	assert.Equal(t, []string{"policies/creds"}, ops[0].Environments)
	assert.Zero(t, client.listCalls)
	assert.Empty(t, client.packGets)
}

func TestPolicyEnable_SameVersionPreservesConfig(t *testing.T) {
	t.Parallel()

	config := rawConfig(t, `{"s3":{"limit":3}}`)
	client := &fakePolicyEnableClient{groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
		Name: "p", VersionTag: "1.2.0", Config: config,
	})}}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p", "1.2.0")
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.JSONEq(t, `{"s3":{"limit":3}}`, configJSON(t, ops[0].Config))
	assert.Empty(t, client.packGets)
}

func TestPolicyEnable_VersionRequiredWhenNotEnabled(t *testing.T) {
	t.Parallel()

	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), &fakePolicyEnableClient{},
		"acme/p", "--policy-group", "prod")
	var cfgErr cmdCmd.ConfigurationError
	require.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, `policy pack "p" is not enabled in policy group "prod"; specify a version or "latest"`, err.Error())
	assert.Empty(t, ops)
}

func TestPolicyEnable_EnforcementLevelClearsOverrides(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{
		groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
			Name: "p", VersionTag: "1.0.0",
			Config: rawConfig(t, `{
				"all": {"enforcementLevel": "advisory"},
				"s3": {"enforcementLevel": "disabled", "limit": 3},
				"iam": "mandatory",
				"ec2": {"allowedTypes": ["t3.micro"]}
			}`),
			Environments: []string{"policies/creds"},
		})},
		policies: map[string][]cloudapitype.AppPolicy{"1.0.0": policiesNamed("s3", "iam", "ec2")},
	}
	var ops []backend.PolicyPackOperation
	_, stderr, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p",
		"--enforcement-level", "mandatory", "--policy-enforcement", "ec2=advisory")
	require.NoError(t, err)
	assert.Empty(t, stderr)

	require.Len(t, ops, 1)
	assert.JSONEq(t, `{
		"all": {"enforcementLevel": "mandatory"},
		"s3": {"limit": 3},
		"ec2": {"allowedTypes": ["t3.micro"], "enforcementLevel": "advisory"}
	}`, configJSON(t, ops[0].Config))
	assert.Equal(t, []string{"policies/creds"}, ops[0].Environments)
	assert.Equal(t, []string{"1.0.0"}, client.packGets)
}

func TestPolicyEnable_PolicyEnforcementOnlyKeepsOtherPolicies(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{
		groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
			Name: "p", VersionTag: "1.0.0",
			Config: rawConfig(t, `{"all":{"enforcementLevel":"advisory"},"s3":{"enforcementLevel":"disabled","limit":3}}`),
		})},
		policies: map[string][]cloudapitype.AppPolicy{"1.0.0": policiesNamed("s3", "iam")},
	}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p",
		"--policy-enforcement", "iam=mandatory")
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.JSONEq(t, `{
		"all": {"enforcementLevel": "advisory"},
		"s3": {"enforcementLevel": "disabled", "limit": 3},
		"iam": {"enforcementLevel": "mandatory"}
	}`, configJSON(t, ops[0].Config))
}

func TestPolicyEnable_UnknownPolicyIsConfigurationError(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{
		groups:   []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{Name: "p", VersionTag: "1.0.0"})},
		policies: map[string][]cloudapitype.AppPolicy{"1.0.0": policiesNamed("s3")},
	}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p",
		"--policy-enforcement", "nope=mandatory")
	var cfgErr cmdCmd.ConfigurationError
	require.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, `invalid --policy-enforcement: policy "nope" is not in version 1.0.0 of policy pack "p"`, err.Error())
	assert.Empty(t, ops)
}

func TestPolicyEnable_VersionChangeDropsMissingPolicies(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{
		groups: []cloudapitype.PolicyGroup{
			groupWith(cloudapitype.AppPolicyPackMetadata{
				Name: "p", VersionTag: "1.0.0",
				Config: rawConfig(t, `{
					"all": {"enforcementLevel": "mandatory"},
					"kept": {"limit": 3},
					"removed-a": {"enforcementLevel": "advisory"},
					"removed-b": "disabled"
				}`),
			}),
			groupWith(cloudapitype.AppPolicyPackMetadata{
				Name: "p", VersionTag: "2.0.0",
				Config: rawConfig(t, `{"all":{"enforcementLevel":"mandatory"},"kept":{"limit":3}}`),
			}),
		},
		policies: map[string][]cloudapitype.AppPolicy{"2.0.0": policiesNamed("kept", "added")},
	}
	var ops []backend.PolicyPackOperation
	out, stderr, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p", "2.0.0", "--output", "json")
	require.NoError(t, err)

	require.Len(t, ops, 1)
	assert.Equal(t, "2.0.0", *ops[0].VersionTag)
	assert.JSONEq(t, `{"all":{"enforcementLevel":"mandatory"},"kept":{"limit":3}}`, configJSON(t, ops[0].Config))
	assert.Equal(t,
		"warning: removed configuration for policies not in version 2.0.0 of policy pack \"p\": removed-a, removed-b\n",
		stderr)
	assert.Equal(t, []string{"2.0.0"}, client.packGets)

	var got cloudapitype.PolicyGroup
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got.AppliedPolicyPacks, 1)
	assert.Equal(t, "2.0.0", got.AppliedPolicyPacks[0].VersionTag)
}

func TestPolicyEnable_LatestResolvesVersionForDrop(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{
		groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
			Name: "p", VersionTag: "1.0.0", Config: rawConfig(t, `{"gone":{"limit":1},"kept":{"limit":2}}`),
		})},
		packs: []cloudapitype.AppPolicyPackWithVersions{
			{Name: "p", Versions: []int{1, 2}, VersionTags: []string{"1.0.0", "2.0.0"}},
		},
		policies: map[string][]cloudapitype.AppPolicy{"2.0.0": policiesNamed("kept")},
	}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p", "latest")
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Nil(t, ops[0].VersionTag)
	assert.JSONEq(t, `{"kept":{"limit":2}}`, configJSON(t, ops[0].Config))
	assert.Equal(t, 1, client.listCalls)
}

func TestPolicyEnable_ConfigFileReplacesConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"kept":{"enforcementLevel":"mandatory"}}`), 0o600))

	client := &fakePolicyEnableClient{groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
		Name: "p", VersionTag: "1.0.0",
		Config:       rawConfig(t, `{"other":{"limit":1}}`),
		Environments: []string{"policies/creds"},
	})}}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p", "2.0.0",
		"--config", path, "--environment", "policies/extra")
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.JSONEq(t, `{"kept":{"enforcementLevel":"mandatory"}}`, configJSON(t, ops[0].Config))
	assert.Equal(t, []string{"policies/creds", "policies/extra"}, ops[0].Environments)
	assert.Empty(t, client.packGets, "configuration from --config is not filtered")
}

func TestPolicyEnable_Environments(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
		Name: "p", VersionTag: "1.0.0", Environments: []string{"a/one@1", "a/two", "a/three@v2"},
	})}}
	var ops []backend.PolicyPackOperation
	_, _, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p",
		"--environment", "a/one@1", // already attached: no-op
		"--environment", "a/new",
		"--remove-environment", "a/missing", // not attached: no-op
		"--remove-environment", "a/three", // any version
	)
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, []string{"a/one@1", "a/two", "a/new"}, ops[0].Environments)
	assert.Nil(t, ops[0].Config)
}

func TestPolicyEnable_RemediateWarnsOnStderr(t *testing.T) {
	t.Parallel()

	client := &fakePolicyEnableClient{
		groups: []cloudapitype.PolicyGroup{groupWith(cloudapitype.AppPolicyPackMetadata{
			Name: "p", VersionTag: "1.0.0", Config: rawConfig(t, `{"all":{"enforcementLevel":"remediate"}}`),
		})},
		policies: map[string][]cloudapitype.AppPolicy{"1.0.0": policiesNamed("s3")},
	}
	var ops []backend.PolicyPackOperation
	out, stderr, err := runPolicyEnable(t, recordingPolicyPack("p", &ops), client, "acme/p",
		"--enforcement-level", "remediate", "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, remediateWarning, stderr)
	assert.NotContains(t, out, "warning")
	assert.True(t, json.Valid([]byte(out)))
}

func TestPolicyEnable_MalformedFlagsAreConfigurationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			"bad level",
			[]string{"--enforcement-level", "strict"},
			`invalid --enforcement-level "strict": must be one of advisory, mandatory, remediate, disabled`,
		},
		{
			"missing equals",
			[]string{"--policy-enforcement", "s3"},
			`invalid --policy-enforcement "s3": expected <policy>=<level>`,
		},
		{
			"empty policy",
			[]string{"--policy-enforcement", "=mandatory"},
			`invalid --policy-enforcement "=mandatory": expected <policy>=<level>`,
		},
		{
			"bad policy level",
			[]string{"--policy-enforcement", "s3=loud"},
			`invalid --policy-enforcement level "loud": must be one of advisory, mandatory, remediate, disabled`,
		},
		{
			"all key",
			[]string{"--policy-enforcement", "all=mandatory"},
			`invalid --policy-enforcement "all=mandatory": use --enforcement-level to set the level of every policy`,
		},
		{
			"conflicting duplicate",
			[]string{"--policy-enforcement", "s3=mandatory", "--policy-enforcement", "s3=advisory"},
			`conflicting --policy-enforcement values for policy "s3": "mandatory" and "advisory"`,
		},
		{
			"bad environment",
			[]string{"--environment", "creds"},
			`invalid --environment "creds": expected <project>/<env>[@<version|tag>]`,
		},
		{
			"empty environment version",
			[]string{"--remove-environment", "a/b@"},
			`invalid --remove-environment "a/b@": expected <project>/<env>[@<version|tag>]`,
		},
		{
			"add and remove same environment",
			[]string{"--environment", "a/b@2", "--remove-environment", "a/b"},
			`environment "a/b" is both attached with --environment and detached with --remove-environment`,
		},
		{
			"config with level",
			[]string{"--config", "c.json", "--enforcement-level", "mandatory"},
			`--config cannot be combined with --enforcement-level or --policy-enforcement`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			depsCalled := false
			deps := func(context.Context, string) (backend.PolicyPack, policyEnableClient, error) {
				depsCalled = true
				return nil, nil, errors.New("unreachable")
			}
			cmd := newPolicyEnableCmdWith(deps)
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append([]string{"acme/p", "latest"}, tt.args...))
			err := cmd.ExecuteContext(t.Context())

			var cfgErr cmdCmd.ConfigurationError
			require.ErrorAs(t, err, &cfgErr)
			assert.Equal(t, tt.want, err.Error())
			assert.False(t, depsCalled)
		})
	}
}

func TestPolicyEnable_HelpListsLevelsAndExamples(t *testing.T) {
	t.Parallel()

	cmd := newPolicyEnableCmdWith(stubPolicyEnableDeps(nil, nil))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
	help := out.String()
	assert.Contains(t, help, "One of: advisory, mandatory, remediate, disabled")
	assert.Contains(t, help, "--policy-enforcement")
	assert.Contains(t, help, "--remove-environment")
	assert.Contains(t, help, "pulumi policy enable acme/aws-guardrails --policy-group prod")
}
