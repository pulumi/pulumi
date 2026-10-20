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
		return cloudapitype.PolicyGroup{AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{
			{Name: "other", VersionTag: "9.9.9"},
			{Name: "aws-guardrails", Version: 3, VersionTag: "1.2.0", Config: map[string]*json.RawMessage{"all": &raw}},
		}}, nil
	}}

	out, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/aws-guardrails", "1.2.0", "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "", enabledGroup)
	assert.Equal(t, "acme", gotOrg)
	assert.Equal(t, "default-policy-group", gotGroup)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "acme", got["organization"])
	assert.Equal(t, "default-policy-group", got["policyGroup"])
	pp := got["policyPack"].(map[string]any)
	assert.Equal(t, "aws-guardrails", pp["name"])
	assert.Equal(t, "1.2.0", pp["versionTag"])
	assert.Equal(t, map[string]any{"all": "mandatory"}, pp["config"])
}

func TestPolicyEnable_JSONExplicitGroup(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	client := stubPolicyGroupClient{get: func(_ context.Context, _, group string) (cloudapitype.PolicyGroup, error) {
		assert.Equal(t, "prod", group)
		return cloudapitype.PolicyGroup{AppliedPolicyPacks: []cloudapitype.AppPolicyPackMetadata{{Name: "p"}}}, nil
	}}
	out, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/p", "latest", "--policy-group", "prod", "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"policyGroup": "prod"`)
}

func TestPolicyEnable_JSONPackMissingAfterEnable(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	client := stubPolicyGroupClient{get: func(context.Context, string, string) (cloudapitype.PolicyGroup, error) {
		return cloudapitype.PolicyGroup{}, nil
	}}
	_, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)),
		"acme/aws-guardrails", "latest", "--output", "json")
	require.EqualError(t, err,
		`policy pack "aws-guardrails" not found in policy group "default-policy-group" after enabling`)
}

func TestPolicyEnable_JSONReadBackFailure(t *testing.T) {
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
	require.EqualError(t, err,
		`policy pack "aws-guardrails" was enabled, but reading back policy group "default-policy-group" failed: boom`)
	assert.NotContains(t, out, `"organization"`)
}

func TestPolicyEnable_DefaultNoGroupRead(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	client := stubPolicyGroupClient{get: func(context.Context, string, string) (cloudapitype.PolicyGroup, error) {
		t.Fatal("GetPolicyGroup must not be called without --output json")
		return cloudapitype.PolicyGroup{}, nil
	}}
	out, err := runPolicyPackCmd(t, newPolicyEnableCmdWith(stubPolicyPackDeps(pack, client)), "acme/p", "latest")
	require.NoError(t, err)
	assert.Empty(t, out)
}
