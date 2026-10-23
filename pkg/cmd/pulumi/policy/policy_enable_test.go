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
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
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
	var gotOrg, gotGroup string
	client := stubPolicyGroupClient{get: func(_ context.Context, org, group string) (cloudapitype.PolicyGroup, error) {
		gotOrg, gotGroup = org, group
		raw := json.RawMessage(`"mandatory"`)
		return cloudapitype.PolicyGroup{Name: group, AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{
			{Name: "aws-guardrails", Version: 3, VersionTag: "1.2.0", Config: map[string]*json.RawMessage{"all": &raw}},
		}}, nil
	}}

	out, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/aws-guardrails", "1.2.0", "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "", enabledGroup)
	assert.Equal(t, "acme", gotOrg)
	assert.Equal(t, "default-policy-group", gotGroup)

	var got cloudapitype.PolicyGroup
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "default-policy-group", got.Name)
	require.Len(t, got.AppliedPolicyPacks, 1)
	assert.Equal(t, "aws-guardrails", got.AppliedPolicyPacks[0].Name)
	assert.Equal(t, "1.2.0", got.AppliedPolicyPacks[0].VersionTag)
	assert.JSONEq(t, `"mandatory"`, string(*got.AppliedPolicyPacks[0].Config["all"]))
}

func TestPolicyEnable_ExplicitGroup(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	client := stubPolicyGroupClient{get: func(_ context.Context, _, group string) (cloudapitype.PolicyGroup, error) {
		assert.Equal(t, "prod", group)
		return cloudapitype.PolicyGroup{Name: group}, nil
	}}
	_, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/p", "latest", "--policy-group", "prod", "--output", "json")
	require.NoError(t, err)
}

func TestPolicyEnable_ReadBackFailure(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	client := stubPolicyGroupClient{get: func(context.Context, string, string) (cloudapitype.PolicyGroup, error) {
		return cloudapitype.PolicyGroup{}, errors.New("boom")
	}}
	out, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/aws-guardrails", "latest", "--output", "json")
	require.EqualError(t, err, `reading policy group "default-policy-group" after update: boom`)
	assert.NotContains(t, out, `"name"`)
}

func TestPolicyEnable_DefaultRendersGroup(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	group := cloudapitype.PolicyGroup{
		Name:               "prod",
		AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{{Name: "p", Version: 3, VersionTag: "1.2.0"}},
	}
	client := stubPolicyGroupClient{get: func(context.Context, string, string) (cloudapitype.PolicyGroup, error) {
		return group, nil
	}}
	out, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/p", "latest", "--policy-group", "prod")
	require.NoError(t, err)

	var want bytes.Buffer
	require.NoError(t, renderPolicyGroupGetText(&want, group))
	assert.Equal(t, want.String(), out)
}
