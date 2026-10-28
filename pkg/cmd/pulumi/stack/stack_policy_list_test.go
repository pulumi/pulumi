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

package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type mockStackPolicyListClient struct {
	groups   apitype.ListPolicyGroupsResponse
	packs    map[apitype.PolicyGroupMode]apitype.GetStackPolicyPacksResponse
	groupErr error
	packErr  error

	gotStacks []client.StackIdentifier
	gotModes  []apitype.PolicyGroupMode
}

func (m *mockStackPolicyListClient) GetStackPolicyGroups(
	_ context.Context, stackID client.StackIdentifier,
) (apitype.ListPolicyGroupsResponse, error) {
	m.gotStacks = append(m.gotStacks, stackID)
	return m.groups, m.groupErr
}

func (m *mockStackPolicyListClient) GetStackPolicyPacksForMode(
	_ context.Context, stackID client.StackIdentifier, mode apitype.PolicyGroupMode,
) (apitype.GetStackPolicyPacksResponse, error) {
	m.gotStacks = append(m.gotStacks, stackID)
	m.gotModes = append(m.gotModes, mode)
	return m.packs[mode], m.packErr
}

var stackPolicyTestStackID = client.StackIdentifier{
	Owner:   "acme",
	Project: "web",
	Stack:   tokens.MustParseStackName("prod"),
}

func sampleStackPolicyListClient() *mockStackPolicyListClient {
	return &mockStackPolicyListClient{
		groups: apitype.ListPolicyGroupsResponse{PolicyGroups: []apitype.PolicyGroupSummary{
			{
				Name:                  "prod-policies",
				NumStacks:             4,
				EntityType:            apitype.Stacks,
				Mode:                  apitype.PolicyGroupModePreventative,
				NumEnabledPolicyPacks: 1,
			},
			{
				Name:                  "default-audit",
				IsOrgDefault:          true,
				NumStacks:             10,
				NumAccounts:           2,
				EntityType:            apitype.Stacks,
				Mode:                  apitype.PolicyGroupModeAudit,
				NumEnabledPolicyPacks: 1,
			},
		}},
		packs: map[apitype.PolicyGroupMode]apitype.GetStackPolicyPacksResponse{
			apitype.PolicyGroupModePreventative: {RequiredPolicies: []apitype.RequiredPolicy{{
				Name:         "aws-guardrails",
				DisplayName:  "AWS Guardrails",
				Version:      3,
				VersionTag:   "1.2.0",
				PackLocation: "https://example.com/signed-download",
				Config: map[string]*json.RawMessage{
					"all": new(json.RawMessage(`"mandatory"`)),
				},
				Environments: []string{"infra/policy-secrets"},
			}}},
			apitype.PolicyGroupModeAudit: {RequiredPolicies: []apitype.RequiredPolicy{{
				Name:        "tagging",
				DisplayName: "Tagging",
				Version:     1,
			}}},
		},
	}
}

func TestStackPolicyList_JSON(t *testing.T) {
	t.Parallel()

	c := sampleStackPolicyListClient()
	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}
	require.NoError(t, args.output.Set("json"))

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf, c, stackPolicyTestStackID, args))

	assert.Equal(t, []apitype.PolicyGroupMode{
		apitype.PolicyGroupModePreventative, apitype.PolicyGroupModeAudit,
	}, c.gotModes)
	for _, id := range c.gotStacks {
		assert.Equal(t, stackPolicyTestStackID, id)
	}
	assert.JSONEq(t, `{
		"stack": "acme/web/prod",
		"policyGroups": [
			{
				"name": "prod-policies", "isOrgDefault": false, "numStacks": 4,
				"entityType": "stacks", "mode": "preventative", "numEnabledPolicyPacks": 1
			},
			{
				"name": "default-audit", "isOrgDefault": true, "numStacks": 10, "numAccounts": 2,
				"entityType": "stacks", "mode": "audit", "numEnabledPolicyPacks": 1
			}
		],
		"policyPacks": {
			"preventative": [{
				"name": "aws-guardrails", "displayName": "AWS Guardrails", "version": 3, "versionTag": "1.2.0",
				"packLocation": "https://example.com/signed-download",
				"config": {"all": "mandatory"}, "environments": ["infra/policy-secrets"]
			}],
			"audit": [{"name": "tagging", "displayName": "Tagging", "version": 1, "versionTag": ""}]
		}
	}`, buf.String())
}

func TestStackPolicyList_JSON_Empty(t *testing.T) {
	t.Parallel()

	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}
	require.NoError(t, args.output.Set("json"))

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf,
		&mockStackPolicyListClient{}, stackPolicyTestStackID, args))
	assert.JSONEq(t, `{"stack": "acme/web/prod", "policyGroups": [], "policyPacks": {"preventative": [], "audit": []}}`,
		buf.String())
}

func TestStackPolicyList_Table(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf,
		sampleStackPolicyListClient(), stackPolicyTestStackID,
		stackPolicyListArgs{output: defaultStackPolicyListOutput()}))

	var rows [][]string
	for line := range strings.Lines(buf.String()) {
		rows = append(rows, strings.Fields(line))
	}
	assert.Equal(t, [][]string{
		{"POLICY", "GROUP", "MODE", "DEFAULT"},
		{"prod-policies", "preventative", "N"},
		{"default-audit", "audit", "Y"},
		{},
		{"POLICY", "PACK", "VERSION", "MODE"},
		{"aws-guardrails", "1.2.0", "preventative"},
		{"tagging", "v1", "audit"},
	}, rows)
}

func TestStackPolicyList_Table_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, runStackPolicyList(t.Context(), &buf,
		&mockStackPolicyListClient{}, stackPolicyTestStackID,
		stackPolicyListArgs{output: defaultStackPolicyListOutput()}))

	assert.Equal(t, `No Policy Groups apply to stack acme/web/prod.

No Policy Packs apply to stack acme/web/prod.
`, buf.String())
}

func TestStackPolicyList_Errors(t *testing.T) {
	t.Parallel()

	args := stackPolicyListArgs{output: defaultStackPolicyListOutput()}

	err := runStackPolicyList(t.Context(), &bytes.Buffer{},
		&mockStackPolicyListClient{groupErr: errors.New("groups boom")}, stackPolicyTestStackID, args)
	assert.EqualError(t, err, "groups boom")

	err = runStackPolicyList(t.Context(), &bytes.Buffer{},
		&mockStackPolicyListClient{packErr: errors.New("packs boom")}, stackPolicyTestStackID, args)
	assert.EqualError(t, err, "packs boom")
}

// mockCloudStack adds the httpstate.Stack methods to backend.MockStack.
type mockCloudStack struct {
	backend.MockStack
}

func (s *mockCloudStack) CurrentOperation() *apitype.OperationStatus { return nil }

func (s *mockCloudStack) StackIdentifier() client.StackIdentifier { return stackPolicyTestStackID }

func newStackPolicyListServer(t *testing.T, c *mockStackPolicyListClient) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp any
		var err error
		switch r.URL.Path {
		case "/api/stacks/acme/web/prod/policygroups":
			resp, err = c.GetStackPolicyGroups(r.Context(), stackPolicyTestStackID)
		case "/api/stacks/acme/web/prod/policypacks":
			mode := apitype.PolicyGroupMode(r.URL.Query().Get("mode"))
			resp, err = c.GetStackPolicyPacksForMode(r.Context(), stackPolicyTestStackID, mode)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)
	return server
}

func newStackPolicyListTestLoginManager(be backend.Backend, loginErr error) cmdBackend.LoginManager {
	return &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool, bool, colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return be, loginErr
		},
	}
}

func TestStackPolicyList_LoginError(t *testing.T) {
	t.Parallel()

	cmd := newStackPolicyListCmd(&pkgWorkspace.MockContext{},
		newStackPolicyListTestLoginManager(nil, errors.New("not logged in")))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"-s", "acme/web/prod"})
	assert.ErrorContains(t, cmd.ExecuteContext(t.Context()), "not logged in")
}

func TestStackPolicyList_Flags(t *testing.T) {
	t.Parallel()

	var gotStack string
	server := newStackPolicyListServer(t, sampleStackPolicyListClient())
	be := &httpstate.MockHTTPBackend{
		FClient: func() *client.Client { return client.NewClient(server.URL, "", false, nil) },
	}
	stack := &mockCloudStack{MockStack: backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("prod"), ProjectV: "web"}
		},
		OrgNameF: func() string { return "acme" },
		BackendF: func() backend.Backend { return be },
	}}
	be.ParseStackReferenceF = func(s string) (backend.StackReference, error) {
		gotStack = s
		return stack.Ref(), nil
	}
	be.GetStackF = func(context.Context, backend.StackReference) (backend.Stack, error) {
		return stack, nil
	}

	cmd := newStackPolicyListCmd(&pkgWorkspace.MockContext{}, newStackPolicyListTestLoginManager(be, nil))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"-s", "acme/web/prod", "--output", "json"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, "acme/web/prod", gotStack)
	var out stackPolicyListResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.Equal(t, "acme/web/prod", out.Stack)
}
