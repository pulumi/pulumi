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

//nolint:paralleltest // uses t.Setenv
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
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	cmd := newPolicyLsCmd(ws, lm)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "default-org", queriedOrgName)
}

//nolint:paralleltest // uses t.Setenv
func TestPolicyLsCmd_RespectsLocalDefaultOrgSetting(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_BACKEND_URL", "https://api.pulumi.com")
	require.NoError(t, workspace.SetBackendConfigDefaultOrg("https://api.pulumi.com", "local-default-org"))

	var queriedOrgName string
	be := &backend.MockBackend{
		GetDefaultOrgF: func(context.Context) (string, error) {
			t.Fatal("GetDefaultOrg should not be called when local default org is configured")
			return "", nil
		},
		ListPolicyPacksF: func(
			_ context.Context, orgName string, _ backend.ContinuationToken,
		) (apitype.ListPolicyPacksResponse, backend.ContinuationToken, error) {
			queriedOrgName = orgName
			return apitype.ListPolicyPacksResponse{}, nil, nil
		},
	}

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	cmd := newPolicyLsCmd(ws, lm)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--json"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "local-default-org", queriedOrgName)
}
