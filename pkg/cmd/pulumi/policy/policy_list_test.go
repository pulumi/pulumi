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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestPolicyLsCmd_QueriesPolicyPacksByOrgNameNotUserName(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	var queriedOrgName string
	be := &backend.MockBackend{
		GetDefaultOrgF: func(context.Context) (string, error) {
			return "default-org", nil
		},
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			t.Fatal("CurrentUser should not be called when resolving org for policy ls")
			return "", nil, nil, nil
		},
		ListPolicyPacksF: func(
			_ context.Context, orgName string, _ backend.ContinuationToken,
		) (apitype.ListPolicyPacksResponse, backend.ContinuationToken, error) {
			queriedOrgName = orgName
			return apitype.ListPolicyPacksResponse{}, nil, nil
		},
	}

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(_ string) (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool, bool, colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	cmd := newPolicyListCmd(ws, lm)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "default-org", queriedOrgName)
}

func samplePolicyPacks() []apitype.PolicyPackWithVersions {
	return []apitype.PolicyPackWithVersions{
		{
			Name:        "aws-guardrails",
			DisplayName: "AWS Guardrails",
			Versions:    []int{1, 2, 3},
			VersionTags: []string{"1.0.0", "2.0.0", "3.1.0"},
		},
		{Name: "tagging", DisplayName: "Tagging", Versions: []int{7}, VersionTags: []string{"7.0.0"}},
	}
}

func TestPolicyList_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, formatPolicyPacksConsole(&buf, samplePolicyPacks()))
	out := buf.String()

	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "VERSIONS")
	assert.Contains(t, out, "aws-guardrails")
	assert.Contains(t, out, "1.0.0, 2.0.0, 3.1.0")
	assert.Contains(t, out, "tagging")
	assert.Contains(t, out, "7.0.0")
}

func TestPolicyList_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, formatPolicyPacksJSON(&buf, samplePolicyPacks()))
	assert.JSONEq(t, `[
		{"name": "aws-guardrails", "versions": ["1.0.0", "2.0.0", "3.1.0"]},
		{"name": "tagging", "versions": ["7.0.0"]}
	]`, buf.String())
}
