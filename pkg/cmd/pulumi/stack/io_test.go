// Copyright 2024, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestStackLoadOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give       LoadOption
		offerNew   bool
		setCurrent bool
	}{
		{LoadOnly, false, false},
		{OfferNew, true, false},
		{SetCurrent, false, true},
		{OfferNew | SetCurrent, true, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.give), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				tt.offerNew, tt.give.OfferNew(),
				"OfferNew did not match")
			assert.Equal(t,
				tt.setCurrent, tt.give.SetCurrent(),
				"SetCurrent did not match")
		})
	}
}

// Tests that CreateStack will send an appropriate initial state when it is asked to create a stack with a non-default
// secrets manager.
func TestCreateStack_InitialisesStateWithSecretsManager(t *testing.T) {
	t.Parallel()

	// Arrange.
	_, expectedSm, err := passphrase.NewPassphraseSecretsManager("test-passphrase")
	require.NoError(t, err)

	var actualDeployment apitype.DeploymentV3

	mockBackend := &backend.MockBackend{
		NameF: func() string {
			return "mock"
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
			err := json.Unmarshal(initialState.Deployment, &actualDeployment)
			require.NoError(t, err)
			return &backend.MockStack{RefF: func() backend.StackReference { return ref }}, nil
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return expectedSm, nil
		},
	}

	stackRef := &backend.MockStackReference{StringV: "dev"}

	// Act.
	//nolint:errcheck
	CreateStack(
		t.Context(),
		cmdutil.Diag(),
		pkgWorkspace.Instance,
		mockBackend,
		stackRef,
		"", /*root*/
		CreateStackOptions{},
	)

	// Assert.
	assert.Equal(t, expectedSm.State(), actualDeployment.SecretsProviders.State)
}

// Tests that CreateStack announces the new stack unless Quiet is set.
func TestCreateStackQuiet(t *testing.T) {
	t.Parallel()

	for _, quiet := range []bool{false, true} {
		t.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			t.Parallel()

			// Arrange.
			var buf bytes.Buffer
			sink := diag.DefaultSink(&buf, io.Discard, diag.FormatOptions{Color: colors.Never})

			mockBackend := &backend.MockBackend{
				NameF: func() string {
					return "mock"
				},
				CreateStackF: func(
					ctx context.Context,
					ref backend.StackReference,
					projectRoot string,
					initialState *apitype.UntypedDeployment,
					opts *backend.CreateStackOptions,
				) (backend.Stack, error) {
					return &backend.MockStack{RefF: func() backend.StackReference { return ref }}, nil
				},
				DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
					return nil, nil
				},
			}

			// Act.
			_, err := CreateStack(
				t.Context(),
				sink,
				pkgWorkspace.Instance,
				mockBackend,
				&backend.MockStackReference{StringV: "quietstack"},
				"", /*root*/
				CreateStackOptions{Quiet: quiet},
			)

			// Assert.
			require.NoError(t, err)
			if quiet {
				assert.NotContains(t, buf.String(), "Created stack")
			} else {
				assert.Contains(t, buf.String(), "Created stack 'quietstack'")
			}
		})
	}
}

func newManagedTestBackend(managedEnv string, createdManaged *bool) *backend.MockManagedEnvironmentsBackend {
	return &backend.MockManagedEnvironmentsBackend{MockEnvironmentsBackend: backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF:              func() string { return "mock" },
			ValidateStackNameF: func(string) error { return nil },
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) {
				return nil, nil
			},
			CreateStackF: func(
				_ context.Context,
				ref backend.StackReference,
				_ string,
				_ *apitype.UntypedDeployment,
				opts *backend.CreateStackOptions,
			) (backend.Stack, error) {
				*createdManaged = opts.ManagedEnvironment
				return &backend.MockStack{
					RefF: func() backend.StackReference { return ref },
					ConfigLocationF: func() backend.StackConfigLocation {
						return backend.StackConfigLocation{ManagedEnvironment: managedEnv}
					},
				}, nil
			},
			DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
				return nil, nil
			},
		},
	}}
}

func TestCreateStackManagedEnvironmentUnsupportedBackend(t *testing.T) {
	t.Parallel()

	stackFile := filepath.Join(t.TempDir(), "Pulumi.dev.yaml")
	b := &backend.MockBackend{
		NameF: func() string { return "diy" },
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			t.Error("the secrets manager must not be set up")
			return nil, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			t.Error("the stack file must not be read")
			return nil, "", errors.New("unexpected call")
		},
	}

	_, err := CreateStack(t.Context(), diagtest.LogSink(t), ws, b,
		&backend.MockStackReference{StringV: "dev", NameV: tokens.MustParseStackName("dev")}, "",
		CreateStackOptions{ConfigFile: stackFile, ManagedEnvironment: true})
	require.ErrorIs(t, err, backend.ErrManagedEnvironmentNotSupported)
	assert.NoFileExists(t, stackFile)
}

func TestCreateStackManagedEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		stackFile   string
		contents    string
		managedEnv  string
		secrets     string
		quiet       bool
		wantCreated bool
		wantErr     string
		wantErrIs   error
	}{
		{name: "created", stackFile: "Pulumi.dev.yaml", managedEnv: "proj/renamed", wantCreated: true},
		{name: "quiet", stackFile: "Pulumi.dev.yaml", managedEnv: "proj/renamed", quiet: true, wantCreated: true},
		{name: "JSON stack file", stackFile: "Pulumi.dev.json", managedEnv: "proj/renamed", wantErr: managedRequiresYAML},
		{
			name:       "existing config",
			stackFile:  "Pulumi.dev.yaml",
			contents:   "config:\n  proj:a: b\n",
			managedEnv: "proj/renamed",
			wantErr: "already has config or an environment; delete its `config` and `environment` sections, " +
				"or the file, before creating a managed stack",
		},
		{
			name:       "existing config is refused before the secrets provider is set up",
			stackFile:  "Pulumi.dev.yaml",
			contents:   "config:\n  proj:a: b\n",
			managedEnv: "proj/renamed",
			secrets:    "unknown://provider",
			wantErr:    "already has config or an environment",
		},
		{
			name:       "existing environment",
			stackFile:  "Pulumi.dev.yaml",
			contents:   "environment:\n  - other\n",
			managedEnv: "proj/renamed",
			wantErr: "already has config or an environment; delete its `config` and `environment` sections, " +
				"or the file, before creating a managed stack",
		},
		{
			name:       "existing passphrase salt",
			stackFile:  "Pulumi.dev.yaml",
			contents:   "encryptionsalt: v1:abc\n",
			managedEnv: "proj/renamed",
			wantErr: "already has secrets provider settings; delete its `secretsprovider`, `encryptionsalt` and " +
				"`encryptedkey` keys, or the file, before creating a managed stack",
		},
		{
			name:       "existing secrets provider",
			stackFile:  "Pulumi.dev.yaml",
			contents:   "secretsprovider: awskms://alias/x\nencryptedkey: abc\n",
			managedEnv: "proj/renamed",
			wantErr:    "already has secrets provider settings",
		},
		{
			name:        "service without managed environments",
			stackFile:   "Pulumi.dev.yaml",
			wantCreated: true,
			wantErr:     "the stack was created without an ESC environment; remove it with `pulumi stack rm acme/proj/dev`",
			wantErrIs:   ErrCreatedWithoutManagedEnvironment,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			stackFile := filepath.Join(dir, tt.stackFile)
			if tt.contents != "" {
				require.NoError(t, os.WriteFile(stackFile, []byte(tt.contents), 0o600))
			}
			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(string) (*workspace.Project, string, error) {
					return &workspace.Project{Name: "proj"}, filepath.Join(dir, "Pulumi.yaml"), nil
				},
			}
			var out bytes.Buffer
			sink := diag.DefaultSink(&out, io.Discard, diag.FormatOptions{Color: colors.Never})
			var createdManaged bool

			_, err := CreateStack(t.Context(), sink, ws, newManagedTestBackend(tt.managedEnv, &createdManaged),
				&backend.MockStackReference{
					StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "acme/proj/dev",
				}, "",
				CreateStackOptions{
					ConfigFile: stackFile, ManagedEnvironment: true, SecretsProvider: tt.secrets, Quiet: tt.quiet,
				})

			assert.Equal(t, tt.wantCreated, createdManaged)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCreated, StackCreatedDespiteError(err))
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				if tt.contents == "" {
					assert.NoFileExists(t, stackFile)
				}
				return
			}
			require.NoError(t, err)
			if tt.quiet {
				assert.Empty(t, out.String())
			} else {
				assert.Contains(t, out.String(), "Created environment proj/renamed for stack configuration")
			}
		})
	}
}

func TestCreateStackManagedEnvironmentSaveFails(t *testing.T) {
	t.Parallel()

	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o600))
	stackFile := filepath.Join(notADir, "Pulumi.dev.yaml")
	var createdManaged bool

	_, err := CreateStack(t.Context(), diagtest.LogSink(t), &pkgWorkspace.MockContext{},
		newManagedTestBackend("proj/renamed", &createdManaged),
		&backend.MockStackReference{
			StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "acme/proj/dev",
		}, "",
		CreateStackOptions{ConfigFile: stackFile, ManagedEnvironment: true})
	require.ErrorIs(t, err, ErrSaveStackConfig)
	assert.ErrorContains(t, err, "Stack acme/proj/dev and its ESC environment were created; remove them with "+
		"`pulumi stack rm acme/proj/dev`")

	_, err = ManagedEnvironmentDefinition(managedLoc, &workspace.ProjectStack{}, "acme", "dev", stackFile)
	assert.ErrorContains(t, err, ManagedEnvironmentMissingFromFileHint("acme", "proj/renamed", "dev", stackFile))
}

func TestCreateStackManagedEnvironmentSelectFails(t *testing.T) {
	t.Parallel()

	stackFile := filepath.Join(t.TempDir(), "Pulumi.dev.yaml")
	var createdManaged bool
	b := newManagedTestBackend("proj/renamed", &createdManaged)
	b.URLF = func() string { return "https://api.example.com" }
	ws := &pkgWorkspace.MockContext{
		NewF: func(string) (pkgWorkspace.W, error) { return nil, errors.New("workspace settings are read-only") },
	}

	_, err := CreateStack(t.Context(), diagtest.LogSink(t), ws, b,
		&backend.MockStackReference{
			StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "acme/proj/dev",
		}, "",
		CreateStackOptions{ConfigFile: stackFile, ManagedEnvironment: true, SetCurrent: true})

	require.ErrorIs(t, err, ErrSelectCreatedStack)
	assert.ErrorContains(t, err, "workspace settings are read-only. Stack acme/proj/dev and its ESC environment were "+
		"created; remove them with `pulumi stack rm acme/proj/dev`")
}
