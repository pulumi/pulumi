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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type stubPolicyPackRef struct{ org, name string }

func (r stubPolicyPackRef) String() string     { return r.org + "/" + r.name }
func (r stubPolicyPackRef) OrgName() string    { return r.org }
func (r stubPolicyPackRef) Name() tokens.QName { return tokens.QName(r.name) }

func newTestWsAndLm(be backend.Backend) (pkgWorkspace.Context, cmdBackend.LoginManager) {
	return &pkgWorkspace.MockContext{}, &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool,
		) (backend.Backend, error) {
			return be, nil
		},
	}
}

func newTestBackend(pack backend.PolicyPack) *backend.MockBackend {
	return &backend.MockBackend{
		GetPolicyPackF: func(context.Context, string, diag.Sink) (backend.PolicyPack, error) {
			return pack, nil
		},
		GetDefaultOrgF: func(context.Context) (string, error) {
			return "org1", nil
		},
	}
}

// newTestCloudBackend returns a cloud backend whose client sends API requests to handler.
func newTestCloudBackend(t *testing.T, pack backend.PolicyPack, handler http.HandlerFunc) *httpstate.MockHTTPBackend {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &httpstate.MockHTTPBackend{
		MockBackend: *newTestBackend(pack),
		FClient: func() *client.Client {
			return client.NewClient(srv.URL, "", false, cmdutil.Diag())
		},
	}
}

func respondJSON(t *testing.T, wantPath string, body any) http.HandlerFunc {
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, wantPath, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}

func respondError(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "boom", http.StatusBadRequest)
}

func newPolicyEnableTestCmd(t *testing.T, pack backend.PolicyPack, handler http.HandlerFunc) *cobra.Command {
	return newPolicyEnableCmd(newTestWsAndLm(newTestCloudBackend(t, pack, handler)))
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
	raw := json.RawMessage(`"mandatory"`)
	handler := respondJSON(t, "/api/orgs/acme/policygroups/default-policy-group", apitype.GetPolicyGroupResponse{
		Name: "default-policy-group",
		AppliedPolicyPacks: []apitype.PolicyPackMetadata{
			{Name: "aws-guardrails", Version: 3, VersionTag: "1.2.0", Config: map[string]*json.RawMessage{"all": &raw}},
		},
	})

	out, err := runPolicyPackCmd(t, newPolicyEnableTestCmd(t, pack, handler),
		"acme/aws-guardrails", "1.2.0", "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "", enabledGroup)

	var got apitype.GetPolicyGroupResponse
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
	handler := respondJSON(t, "/api/orgs/acme/policygroups/prod", apitype.GetPolicyGroupResponse{Name: "prod"})
	_, err := runPolicyPackCmd(t, newPolicyEnableTestCmd(t, pack, handler),
		"acme/p", "latest", "--policy-group", "prod", "--output", "json")
	require.NoError(t, err)
}

func TestPolicyEnable_ReadBackFailure(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	out, err := runPolicyPackCmd(t, newPolicyEnableTestCmd(t, pack, respondError),
		"acme/aws-guardrails", "latest", "--output", "json")
	require.ErrorContains(t, err, `reading policy group "default-policy-group" after update: `)
	assert.NotContains(t, out, `"name"`)
}

func TestPolicyEnable_DefaultRendersGroup(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	group := apitype.GetPolicyGroupResponse{
		Name:               "prod",
		AppliedPolicyPacks: []apitype.PolicyPackMetadata{{Name: "p", Version: 3, VersionTag: "1.2.0"}},
	}
	out, err := runPolicyPackCmd(t,
		newPolicyEnableTestCmd(t, pack, respondJSON(t, "/api/orgs/acme/policygroups/prod", group)),
		"acme/p", "latest", "--policy-group", "prod")
	require.NoError(t, err)

	var want bytes.Buffer
	require.NoError(t, renderPolicyGroupGetText(&want, group))
	assert.Equal(t, want.String(), out)
}

func TestPolicyEnable_JSONMatchesGroupGet(t *testing.T) {
	t.Parallel()

	group := samplePolicyGroupResponse()
	group.Name = "default-policy-group"

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	enableOut, err := runPolicyPackCmd(t,
		newPolicyEnableTestCmd(t, pack, respondJSON(t, "/api/orgs/acme/policygroups/default-policy-group", group)),
		"acme/aws-guardrails", "latest", "--output", "json")
	require.NoError(t, err)

	var getOut bytes.Buffer
	getArgs := policyGroupGetArgs{outputFormat: defaultPolicyGroupGetOutputFormat()}
	require.NoError(t, getArgs.outputFormat.Set("json"))
	require.NoError(t, runPolicyGroupGet(t.Context(), &getOut,
		stubPolicyGroupGetFactory(&mockPolicyGroupGetClient{resp: group}, "acme"), group.Name, getArgs))

	assert.JSONEq(t, getOut.String(), enableOut)
}

func TestPolicyEnable_ConfigFileEnvironments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		file             string
		wantEnvironments []string
	}{
		{
			name:             "with environments",
			file:             `{"s3-no-public": "mandatory", "environments": ["policies/aws-creds@3"]}`,
			wantEnvironments: []string{"policies/aws-creds@3"},
		},
		{
			name: "without environments",
			file: `{"s3-no-public": "mandatory"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.file), 0o600))

			var gotOp backend.PolicyPackOperation
			pack := &backend.MockPolicyPack{
				RefF: func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
				EnableF: func(_ context.Context, _ string, op backend.PolicyPackOperation) error {
					gotOp = op
					return nil
				},
			}
			handler := respondJSON(t, "/api/orgs/acme/policygroups/default-policy-group",
				apitype.GetPolicyGroupResponse{Name: "default-policy-group"})

			_, err := runPolicyPackCmd(t, newPolicyEnableTestCmd(t, pack, handler),
				"acme/aws-guardrails", "latest", "--config", path, "--output", "json")
			require.NoError(t, err)

			assert.Equal(t, tt.wantEnvironments, gotOp.Environments)
			require.Contains(t, gotOp.Config, "s3-no-public")
			assert.JSONEq(t, `{"enforcementLevel":"mandatory"}`, string(*gotOp.Config["s3-no-public"]))
		})
	}
}

func TestPolicyEnable_ConfigFileExceptionsWarning(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"version": 2,
		"policies": {"s3-no-public": "mandatory"},
		"exceptions": {"a": {"reason": "r", "match": [
			{"policies": ["s3-no-public"], "targets": ["urn:pulumi:prod::web::pkg:index:R::r"]}
		]}}
	}`), 0o600))

	pack := &backend.MockPolicyPack{
		RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "aws-guardrails"} },
		EnableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	handler := respondJSON(t, "/api/orgs/acme/policygroups/default-policy-group",
		apitype.GetPolicyGroupResponse{Name: "default-policy-group"})

	out, err := runPolicyPackCmd(t, newPolicyEnableTestCmd(t, pack, handler),
		"acme/aws-guardrails", "latest", "--config", path)
	require.NoError(t, err)
	assert.Contains(t, out, "warning: ignoring the exceptions in "+path)
}

func TestLoadPolicyConfigFromFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		file             string
		wantConfig       map[string]string
		wantEnvironments []string
	}{
		{
			name: "unversioned with environments",
			file: `{
				"s3-no-public": "mandatory",
				"required-tags": {"enforcementLevel": "advisory", "tags": ["owner"]},
				"environments": ["policies/aws-creds", "policies/shared@3"]
			}`,
			wantConfig: map[string]string{
				"s3-no-public":  `{"enforcementLevel":"mandatory"}`,
				"required-tags": `{"enforcementLevel":"advisory","tags":["owner"]}`,
			},
			wantEnvironments: []string{"policies/aws-creds", "policies/shared@3"},
		},
		{
			name: "version 2 with environments",
			file: `{
				"version": 2,
				"environments": ["policies/aws-creds"],
				"policies": {"s3-no-public": "mandatory"}
			}`,
			wantConfig:       map[string]string{"s3-no-public": `{"enforcementLevel":"mandatory"}`},
			wantEnvironments: []string{"policies/aws-creds"},
		},
		{
			name:       "without environments",
			file:       `{"s3-no-public": "mandatory"}`,
			wantConfig: map[string]string{"s3-no-public": `{"enforcementLevel":"mandatory"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.file), 0o600))

			file, err := loadPolicyConfigFromFile(path)
			require.NoError(t, err)

			got := make(map[string]string, len(file.config))
			for k, v := range file.config {
				got[k] = string(*v)
			}
			assert.Equal(t, tt.wantConfig, got)
			assert.Equal(t, tt.wantEnvironments, file.environments)
		})
	}
}
