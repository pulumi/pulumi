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
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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

// escConfigBackend extends a fake environment universe with just enough backend to create a stack.
func (u *fakeEnvUniverse) escConfigBackend(t *testing.T) *backend.MockEnvironmentsBackend {
	be := u.backend()
	be.ValidateStackNameF = func(string) error { return nil }
	be.ParseStackReferenceF = func(ref string) (backend.StackReference, error) {
		return &backend.MockStackReference{
			StringV:             ref,
			NameV:               tokens.MustParseStackName(ref),
			FullyQualifiedNameV: tokens.QName("acme/payments/" + ref),
		}, nil
	}
	be.DefaultSecretManagerF = func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
		return nil, nil
	}
	// The stack does not exist until CreateStack makes it.
	be.GetStackF = func(context.Context, backend.StackReference) (backend.Stack, error) {
		return nil, nil
	}
	be.CreateStackF = func(
		_ context.Context, ref backend.StackReference, _ string,
		_ *apitype.UntypedDeployment, _ *backend.CreateStackOptions,
	) (backend.Stack, error) {
		return &backend.MockStack{
			RefF:     func() backend.StackReference { return ref },
			OrgNameF: func() string { return "acme" },
			BackendF: func() backend.Backend { return be },
		}, nil
	}
	return be
}

func escConfigProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.yaml"), []byte("name: payments\nruntime: nodejs\n"), 0o600))
	return dir
}

// A backend that cannot host environments must be refused before anything is created.
//
//nolint:paralleltest // changes directory for the process
func TestStackInitESCConfigRefusesBackendWithoutEnvironments(t *testing.T) {
	t.Chdir(escConfigProjectDir(t))

	mockBackend := &backend.MockBackend{
		NameF:              func() string { return "file://~" },
		ValidateStackNameF: func(string) error { return nil },
		ParseStackReferenceF: func(ref string) (backend.StackReference, error) {
			return &backend.MockStackReference{StringV: ref}, nil
		},
		CreateStackF: func(
			context.Context, backend.StackReference, string,
			*apitype.UntypedDeployment, *backend.CreateStackOptions,
		) (backend.Stack, error) {
			t.Fatal("the stack must not be created when the backend cannot host environments")
			return nil, nil
		},
	}
	cmd := &stackNewCmd{
		stackName: "dev",
		escConfig: true,
		noSelect:  true,
		stdout:    io.Discard,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	}

	err := cmd.Run(t.Context(), nil /* args */)
	assert.ErrorContains(t, err, "backend file://~ does not support environments")
	assert.NoFileExists(t, "Pulumi.dev.yaml")
}

//nolint:paralleltest // changes directory for the process
func TestStackInitESCConfigRefusesCopyConfigFrom(t *testing.T) {
	t.Chdir(escConfigProjectDir(t))

	u := newFakeEnvUniverse(t)
	be := u.escConfigBackend(t)
	be.CreateStackF = func(
		context.Context, backend.StackReference, string,
		*apitype.UntypedDeployment, *backend.CreateStackOptions,
	) (backend.Stack, error) {
		t.Fatal("the stack must not be created when the flags are refused")
		return nil, nil
	}
	cmd := &stackNewCmd{
		stackName:   "dev",
		escConfig:   true,
		stackToCopy: "prod",
		noSelect:    true,
		stdout:      io.Discard,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	err := cmd.Run(t.Context(), nil /* args */)
	assert.ErrorContains(t, err, "--esc-config cannot be combined with --copy-config-from")
}

// With the flag, the stack file points at the new environment and holds no config of its own.
//
//nolint:paralleltest // changes directory for the process
func TestStackInitESCConfigWritesMainEnvironment(t *testing.T) {
	dir := escConfigProjectDir(t)
	t.Chdir(dir)

	u := newFakeEnvUniverse(t)
	var out bytes.Buffer
	cmd := &stackNewCmd{
		stackName: "dev",
		escConfig: true,
		noSelect:  true,
		stdout:    &out,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return u.escConfigBackend(t), nil
		},
	}

	require.NoError(t, cmd.Run(t.Context(), nil /* args */))

	assert.Equal(t, []string{"payments/base", "payments/dev"}, u.created)
	assert.Contains(t, out.String(), "Creating environment 'acme/payments/base'...\n")
	assert.Contains(t, out.String(), "Creating environment 'acme/payments/dev'... (imports payments/base)\n")

	contents, err := os.ReadFile(filepath.Join(dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(contents), "mainEnvironment: payments/dev")
	assert.NotContains(t, string(contents), "config:")
}

// Without the flag nothing about the command changes: the mock's environment functions are never
// reached, and touching them would panic.
//
//nolint:paralleltest // changes directory for the process
func TestStackInitWithoutESCConfigTouchesNoEnvironment(t *testing.T) {
	dir := escConfigProjectDir(t)
	t.Chdir(dir)

	be := (&fakeEnvUniverse{t: t}).escConfigBackend(t)
	be.GetEnvironmentDefinitionF = nil
	be.CreateEnvironmentF = nil
	cmd := &stackNewCmd{
		stackName: "dev",
		noSelect:  true,
		stdout:    io.Discard,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return be, nil
		},
	}

	require.NoError(t, cmd.Run(t.Context(), nil /* args */))
	assert.NoFileExists(t, filepath.Join(dir, "Pulumi.dev.yaml"))
}

// A failure after the stack exists leaves an ordinary, working stack: no 'mainEnvironment' is recorded.
//
//nolint:paralleltest // changes directory for the process
func TestStackInitESCConfigFailureLeavesOrdinaryStack(t *testing.T) {
	dir := escConfigProjectDir(t)
	t.Chdir(dir)

	u := newFakeEnvUniverse(t)
	u.createErrs["payments/dev"] = errors.New("boom")
	cmd := &stackNewCmd{
		stackName: "dev",
		escConfig: true,
		noSelect:  true,
		stdout:    io.Discard,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return u.escConfigBackend(t), nil
		},
	}

	err := cmd.Run(t.Context(), nil /* args */)
	assert.ErrorContains(t, err, "created environment(s) acme/payments/base")
	assert.ErrorContains(t, err, "could not create environment acme/payments/dev: boom")
	assert.ErrorContains(t, err, "the stack was created without 'mainEnvironment'")

	if contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.dev.yaml")); readErr == nil {
		assert.NotContains(t, string(contents), "mainEnvironment")
	}
}
