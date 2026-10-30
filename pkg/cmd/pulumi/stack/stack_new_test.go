// Copyright 2023, Pulumi Corporation.
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
	"context"
	"errors"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When a backend doesn't support the --teams flag,
// stack creation should fail.
func TestStackNew_teamsUnsupportedByBackend(t *testing.T) {
	t.Parallel()

	mockBackend := &backend.MockBackend{
		NameF: func() string {
			return "mock"
		},
		ParseStackReferenceF: func(ref string) (backend.StackReference, error) {
			return &backend.MockStackReference{}, nil
		},
		ValidateStackNameF: func(name string) error {
			assert.Equal(t, "dev", name, "stack name mismatch")
			return nil
		},
		CreateStackF: func(
			ctx context.Context,
			ref backend.StackReference,
			projectRoot string,
			initialState *apitype.UntypedDeployment,
			opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			assert.NotEmpty(t, opts.Teams, "expected teams to be set")
			return nil, backend.ErrTeamsNotSupported
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
	}
	cmd := &stackNewCmd{
		teams:     []string{"red", "blue"},
		stackName: "dev",
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	}

	err := cmd.Run(t.Context(), nil /* args */)
	assert.ErrorContains(t, err, "stack dev uses the mock backend: mock does not support --teams")
}

//nolint:paralleltest // mutates global state
func TestStackNew_RejectsCopyConfigFromManagedStack(t *testing.T) {
	parseRef := func(name string) (backend.StackReference, error) {
		return &backend.MockStackReference{StringV: name, NameV: tokens.MustParseStackName(name)}, nil
	}
	managedStack := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{ManagedEnvironment: "proj/renamed"}
		},
	}
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool,
			colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				ParseStackReferenceF: parseRef,
				GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) {
					return managedStack, nil
				},
			}, nil
		},
	})
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock\n"), 0o600))

	newBackend := &backend.MockBackend{
		ParseStackReferenceF: parseRef,
		ValidateStackNameF:   func(string) error { return nil },
		GetStackF:            func(context.Context, backend.StackReference) (backend.Stack, error) { return nil, nil },
		CreateStackF: func(
			context.Context, backend.StackReference, string, *apitype.UntypedDeployment, *backend.CreateStackOptions,
		) (backend.Stack, error) {
			t.Error("no stack may be created")
			return nil, errors.New("unexpected CreateStack")
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
	}
	cmd := &stackNewCmd{
		stackName:   "dev",
		stackToCopy: "managed",
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return newBackend, nil
		},
	}

	err := cmd.Run(t.Context(), nil /* args */)
	require.EqualError(t, err,
		"`pulumi stack init --copy-config-from` is not supported on stacks with an ESC-managed environment yet")
}

// This test demonstrates that newCreateStackOptions will filter
// out teams consisting exclusively of whitespace. NB: It's not intended
// to fully validate the correctness of team names. For example, it doesn't
// check for illegal punctuation, length, or other measures of correctness.
// To keep the codebase DRY, we pass along team names as-is to the Pulumi Cloud,
// with the exception of trimming whitespace, and allow the Pulumi Cloud to
// validate them.
func TestNewCreateStackOptsFiltersWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		giveTeams []string
		wantTeams []string
	}{
		{
			name: "empty",
			// no raw or valid teams
			giveTeams: []string{},
			wantTeams: []string{},
		},
		{
			name:      "single valid",
			giveTeams: []string{"TeamRocket"},
			wantTeams: []string{"TeamRocket"},
		},
		{
			name:      "all invalid",
			giveTeams: []string{" ", "\t", "\n"},
			wantTeams: []string{},
		},
		{
			name:      "valid and invalid",
			giveTeams: []string{" ", "Edward", "\t", "Jacob", "\n"},
			wantTeams: []string{"Edward", "Jacob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// If the test case provides at least one valid team,
			// then the options should be non-nil.
			got := sanitizeTeams(tt.giveTeams)
			assert.ElementsMatch(t, tt.wantTeams, got)
		})
	}
}
