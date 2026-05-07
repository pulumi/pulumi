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
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestNormalizeBackendURL(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"file with tilde", "file://~", "file://" + home},
		{"file tilde subdir", "file://~/state", "file://" + filepath.Clean(home+"/state")},
		{"file absolute trailing slash", "file:///var/state/", "file:///var/state"},
		{"file absolute clean", "file:///var/./state", "file:///var/state"},
		{"https trailing slash", "https://api.pulumi.com/", "https://api.pulumi.com"},
		{"https no slash", "https://api.pulumi.com", "https://api.pulumi.com"},
		{"s3 trailing slash", "s3://my-bucket/", "s3://my-bucket"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizeBackendURL(tt.in))
		})
	}
}

func runMigrate(
	t *testing.T,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	args []string,
) error {
	t.Helper()
	cmd := newStackMigrateCmd(ws, lm)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.ExecuteContext(t.Context())
}

func TestStackMigrate_RequiresFromFlag(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{}

	err := runMigrate(t, ws, lm, []string{"some-stack"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--from is required")
}

func TestStackMigrate_RejectsInvalidSecretsProvider(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{}

	err := runMigrate(t, ws, lm, []string{
		"--from", "file:///tmp/diy",
		"--secrets-provider", "totally-bogus-provider",
		"some-stack",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown secrets provider")
}

func TestStackMigrate_RejectsBothPositionalAndFlag(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{}

	err := runMigrate(t, ws, lm, []string{
		"--from", "file:///tmp/diy",
		"--source-stack", "from-flag",
		"from-positional",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of --source-stack")
}

func TestStackMigrate_RejectsSameBackend(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	sourceURL := "file://~"
	targetURL := "file://" + home

	be := &backend.MockBackend{
		URLF:  func() string { return targetURL },
		NameF: func() string { return "diy" },
	}
	cmdBackend.BackendInstance = be
	t.Cleanup(func() { cmdBackend.BackendInstance = nil })

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				URLF:  func() string { return sourceURL },
				NameF: func() string { return "diy" },
			}, nil
		},
	}

	err = runMigrate(t, ws, lm, []string{
		"--from", sourceURL,
		"some-stack",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source and target backends are the same")
}

func TestStackMigrate_RejectsExistingTargetStack(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev",
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
	}

	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV:             s,
				NameV:               tokens.MustParseStackName(s),
				FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
	}

	targetBE := &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV:             s,
				NameV:               tokens.MustParseStackName(s),
				FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			// Pretend the target stack already exists.
			return &backend.MockStack{
				RefF: func() backend.StackReference { return ref },
			}, nil
		},
	}
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = nil })

	ws := &pkgWorkspace.MockContext{
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: "dev"}
				},
			}, nil
		},
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Name: "proj"}, "Pulumi.yaml", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return sourceBE, nil
		},
	}

	err := runMigrate(t, ws, lm, []string{
		"--from", "file:///tmp/source",
		"--yes",
		"dev",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestStackMigrate_RollsBackOnImportFailure(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "passphrase" },
		StateF:     func() json.RawMessage { return nil },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev",
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return srcSM, nil
		},
	}
	srcSnapshot := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""}}`),
	}

	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV:             s,
				NameV:               tokens.MustParseStackName(s),
				FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcSnapshot, nil
		},
	}

	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev",
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}

	var (
		tgtCreated bool
		tgtRemoved bool
	)
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		// Backend default returns nil so stack-level default is consulted (mirrors cloud).
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV:             s,
				NameV:               tokens.MustParseStackName(s),
				FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return nil, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			tgtCreated = true
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, _ *apitype.UntypedDeployment) error {
			return errors.New("simulated server-side import failure")
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			tgtRemoved = true
			return false, nil
		},
	}
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = nil })

	ws := &pkgWorkspace.MockContext{
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: "dev"}
				},
			}, nil
		},
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Name: "proj"}, "Pulumi.yaml", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return sourceBE, nil
		},
	}

	err := runMigrate(t, ws, lm, []string{
		"--from", "file:///tmp/source",
		"--yes",
		"dev",
	})
	require.Error(t, err)
	assert.True(t, tgtCreated, "target stack should have been created before the failure")
	assert.True(t, tgtRemoved, "rollback should have removed the partially-created target stack")
}
