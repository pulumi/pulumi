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

// mockPolicyGroupNewClient stubs policyGroupNewClient. It records calls and
// optionally returns fixed responses or errors for each method.
type mockPolicyGroupNewClient struct {
	getResp   cloudapitype.PolicyGroup
	createErr error
	getErr    error

	createdOrg  string
	createdName string
	createdReq  cloudapitype.NewPolicyGroupRequest
	getOrg      string
	getName     string
	createCalls int
	getCalls    int
}

func (m *mockPolicyGroupNewClient) CreatePolicyGroup(
	_ context.Context, org string, req cloudapitype.NewPolicyGroupRequest,
) error {
	m.createCalls++
	m.createdOrg = org
	m.createdName = req.Name
	m.createdReq = req
	return m.createErr
}

func (m *mockPolicyGroupNewClient) GetPolicyGroup(
	_ context.Context, org, name string,
) (cloudapitype.PolicyGroup, error) {
	m.getCalls++
	m.getOrg = org
	m.getName = name
	if m.getErr != nil {
		return cloudapitype.PolicyGroup{}, m.getErr
	}
	return m.getResp, nil
}

func stubPolicyGroupNewFactory(c policyGroupNewClient, org string) policyGroupNewClientFactory {
	return func(_ context.Context, _ string) (policyGroupNewClient, string, error) {
		return c, org, nil
	}
}

func failingPolicyGroupNewFactory(err error) policyGroupNewClientFactory {
	return func(_ context.Context, _ string) (policyGroupNewClient, string, error) {
		return nil, "", err
	}
}

func newPolicyGroupResponseFixture() cloudapitype.PolicyGroup {
	return cloudapitype.PolicyGroup{
		Name:         "prod-policies",
		IsOrgDefault: false,
		EntityType:   cloudapitype.PolicyGroupEntityTypeStacks, //nolint:staticcheck
		Mode:         cloudapitype.PolicyGroupModePreventative,
	}
}

func TestPolicyGroupNew_DefaultOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{mode: "preventative", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `Name:                  prod-policies
Org default:           no
Mode:                  preventative
Applied policy packs:  0
Stacks:                0
Accounts:              0
`, buf.String())
	assert.Equal(t, 1, c.createCalls)
	assert.Equal(t, 1, c.getCalls)
	assert.Equal(t, "acme", c.createdOrg)
	assert.Equal(t, "prod-policies", c.createdName)
	assert.Equal(t, "acme", c.getOrg)
	assert.Equal(t, "prod-policies", c.getName)
}

func TestPolicyGroupNew_JSONOutput(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	args := policyGroupNewArgs{mode: "preventative", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies", args)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"name": "prod-policies",
		"isOrgDefault": false,
		"entityType": "stacks",
		"mode": "preventative",
		"stacks": [],
		"appliedPolicyPacks": [],
		"accounts": []
	}`, buf.String())
}

func TestPolicyGroupNew_CreateError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{createErr: errors.New("already exists")}
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{mode: "preventative", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "creating policy group: already exists", err.Error())
	assert.Equal(t, 1, c.createCalls)
	assert.Equal(t, 0, c.getCalls)
}

func TestPolicyGroupNew_GetAfterCreateError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	c := &mockPolicyGroupNewClient{getErr: errors.New("not found")}
	err := runPolicyGroupNew(t.Context(), buf,
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{mode: "preventative", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "reading policy group after create: not found", err.Error())
	assert.Equal(t, 1, c.createCalls)
	assert.Equal(t, 1, c.getCalls)
}

func TestPolicyGroupNew_FactoryError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	err := runPolicyGroupNew(t.Context(), buf,
		failingPolicyGroupNewFactory(errors.New("not logged in")),
		"prod-policies", policyGroupNewArgs{
			mode: "preventative", yes: true,
			outputFormat: defaultPolicyGroupGetOutputFormat(),
		})
	require.Error(t, err)
	assert.Equal(t, "not logged in", err.Error())
}

func TestPolicyGroupNew_ModeOnly(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{mode: "audit", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, cloudapitype.NewPolicyGroupRequest{
		Name: "prod-policies",
		Mode: new(cloudapitype.PolicyGroupModeAudit),
	}, c.createdReq)
}

func TestPolicyGroupNew_DeprecatedEntityType(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{entityType: "accounts", yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, cloudapitype.NewPolicyGroupRequest{
		Name:       "prod-policies",
		EntityType: new(cloudapitype.PolicyGroupEntityTypeAccounts), //nolint:staticcheck
	}, c.createdReq)
}

func TestPolicyGroupNew_ModeRequired(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{}
	err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{yes: true, outputFormat: defaultPolicyGroupGetOutputFormat()})
	require.Error(t, err)
	assert.Equal(t, "--mode is required (use --mode preventative or --mode audit)", err.Error())
	assert.Equal(t, 0, c.createCalls)
}

func TestPolicyGroupNew_EntityTypeFlagDeprecated(t *testing.T) {
	t.Parallel()

	flag := newPolicyGroupNewCmdWith(stubPolicyGroupNewFactory(nil, "acme")).Flags().Lookup("entity-type")
	require.NotNil(t, flag)
	assert.Equal(t, "use --mode instead", flag.Deprecated)
}

func TestPolicyGroupNew_MatchRules(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{
			mode:         "audit",
			stackRule:    "env=prod OR owner",
			accountRule:  "!temp",
			yes:          true,
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			changed:      map[string]bool{"stack-rule": true, "account-rule": true},
		})
	require.NoError(t, err)

	body, err := json.Marshal(c.createdReq)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"name": "prod-policies",
		"mode": "audit",
		"stackMatchRule": `+binaryJSON("Or",
		tagEqualsJSON(stackCtxJSON, "env", "prod"),
		hasTagJSON(stackCtxJSON, "owner"))+`,
		"accountMatchRule": `+notJSON(hasTagJSON(accountCtxJSON, "temp"))+`
	}`, string(body))
}

func TestPolicyGroupNew_AccountRuleRequiresAudit(t *testing.T) {
	t.Parallel()

	for _, args := range []policyGroupNewArgs{
		{mode: "preventative"},
		{entityType: "stacks"},
	} {
		args.accountRule = "env=prod"
		args.changed = map[string]bool{"account-rule": true}
		args.yes = true
		args.outputFormat = defaultPolicyGroupGetOutputFormat()

		c := &mockPolicyGroupNewClient{}
		err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
			stubPolicyGroupNewFactory(c, "acme"), "prod-policies", args)
		require.Error(t, err)
		assert.Equal(t, cmdCmd.ConfigurationError{Message: "--account-rule requires --mode audit"}, err)
		assert.Equal(t, 0, c.createCalls)
	}
}

func TestPolicyGroupNew_AccountRuleWithDeprecatedAccountsEntityType(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
		stubPolicyGroupNewFactory(c, "acme"), "prod-policies",
		policyGroupNewArgs{
			entityType:   "accounts",
			accountRule:  "env=prod",
			yes:          true,
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			changed:      map[string]bool{"account-rule": true},
		})
	require.NoError(t, err)
	assert.Equal(t, 1, c.createCalls)
	require.NotNil(t, c.createdReq.AccountMatchRule)
}

func TestPolicyGroupNew_InvalidRuleFailsBeforeLogin(t *testing.T) {
	t.Parallel()

	err := runPolicyGroupNew(t.Context(), &bytes.Buffer{},
		failingPolicyGroupNewFactory(errors.New("factory must not be called")), "prod-policies",
		policyGroupNewArgs{
			mode:         "preventative",
			stackRule:    "env=",
			yes:          true,
			outputFormat: defaultPolicyGroupGetOutputFormat(),
			changed:      map[string]bool{"stack-rule": true},
		})
	require.Error(t, err)
	assert.IsType(t, cmdCmd.ConfigurationError{}, err)
	assert.Contains(t, err.Error(), "invalid --stack-rule")
}

func TestPolicyGroupNew_RuleFlagsParse(t *testing.T) {
	t.Parallel()

	c := &mockPolicyGroupNewClient{getResp: newPolicyGroupResponseFixture()}
	cmd := newPolicyGroupNewCmdWith(stubPolicyGroupNewFactory(c, "acme"))
	cmd.SetArgs([]string{
		"prod-policies", "--mode", "audit", "--yes",
		"--stack-rule", "env=prod OR owner",
		"--account-rule", "team=platform",
	})
	cmd.SetOut(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	assert.JSONEq(t,
		binaryJSON("Or", tagEqualsJSON(stackCtxJSON, "env", "prod"), hasTagJSON(stackCtxJSON, "owner")),
		marshalRule(t, c.createdReq.StackMatchRule))
	assert.JSONEq(t, tagEqualsJSON(accountCtxJSON, "team", "platform"),
		marshalRule(t, c.createdReq.AccountMatchRule))
}
