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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakeHTTPStateBackend is a minimal stub that satisfies httpstate.Backend so the
// `targetBE.(httpstate.Backend)` assertion in shouldForceTargetSecretsRewrite resolves true.
// Methods that aren't called by the gating logic panic if invoked; tests using this stub
// should only exercise paths that don't reach them.
type fakeHTTPStateBackend struct {
	*backend.MockBackend
}

func (b *fakeHTTPStateBackend) CloudURL() string { return "https://api.pulumi.com" }

func (b *fakeHTTPStateBackend) StackConsoleURL(_ backend.StackReference) (string, error) {
	return "", nil
}

func (b *fakeHTTPStateBackend) Client() *client.Client { return nil }

func (b *fakeHTTPStateBackend) Capabilities(_ context.Context) apitype.Capabilities {
	return apitype.Capabilities{}
}

func (b *fakeHTTPStateBackend) RunDeployment(
	_ context.Context, _ backend.StackReference, _ apitype.CreateDeploymentRequest,
	_ display.Options, _ string, _ bool,
) error {
	panic("RunDeployment: not implemented in fakeHTTPStateBackend")
}

func (b *fakeHTTPStateBackend) Search(
	_ context.Context, _ string, _ *apitype.PulumiQueryRequest,
) (*apitype.ResourceSearchResponse, error) {
	panic("Search: not implemented in fakeHTTPStateBackend")
}

func (b *fakeHTTPStateBackend) NaturalLanguageSearch(
	_ context.Context, _ string, _ string,
) (*apitype.ResourceSearchResponse, error) {
	panic("NaturalLanguageSearch: not implemented in fakeHTTPStateBackend")
}

func (b *fakeHTTPStateBackend) PromptAI(
	_ context.Context, _ httpstate.AIPromptRequestBody,
) (*http.Response, error) {
	panic("PromptAI: not implemented in fakeHTTPStateBackend")
}

// prefixCrypter is a test crypter that wraps plaintexts with a fixed tag so a test can assert
// which crypter produced which ciphertext.
type prefixCrypter struct{ prefix string }

func (p prefixCrypter) EncryptValue(_ context.Context, plaintext string) (string, error) {
	return p.prefix + ":" + plaintext, nil
}

func (p prefixCrypter) DecryptValue(_ context.Context, ciphertext string) (string, error) {
	rest, ok := strings.CutPrefix(ciphertext, p.prefix+":")
	if !ok {
		return "", fmt.Errorf("decrypt: bad prefix on %q (expected %q)", ciphertext, p.prefix)
	}
	return rest, nil
}

func (p prefixCrypter) BatchEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return config.DefaultBatchEncrypt(ctx, p, plaintexts)
}

func (p prefixCrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return config.DefaultBatchDecrypt(ctx, p, ciphertexts)
}

// restoreConfigFile must return false when the underlying os write fails, so callers know to
// preserve the redundant `.bak` rather than removing it (which could lose the only good copy
// of the source config).
func TestRestoreConfigFile_FailureKeepsCallerInformed(t *testing.T) { //nolint: paralleltest
	dir := t.TempDir()
	// Make `dir/sub` a file so writing under it fails ("not a directory").
	blocker := dir + "/sub"
	require.NoError(t, os.WriteFile(blocker, []byte("blocker"), 0o600))

	var buf strings.Builder
	ok := restoreConfigFile(&buf, blocker+"/cfg.yaml", []byte("orig"), 0o600, true /*existed*/)
	assert.False(t, ok, "restore must report failure when WriteFile errors")
	assert.Contains(t, buf.String(), "failed to restore",
		"warning should mention the restore failure: %s", buf.String())
}

// restoreConfigFile happy path: returns true and writes "Restored ..." line.
func TestRestoreConfigFile_SuccessReportsRestored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/Pulumi.dev.yaml"
	require.NoError(t, os.WriteFile(path, []byte("mutated"), 0o600))

	var buf strings.Builder
	ok := restoreConfigFile(&buf, path, []byte("orig"), 0o600, true)
	assert.True(t, ok)
	got, _ := os.ReadFile(path)
	assert.Equal(t, []byte("orig"), got)
	assert.Contains(t, buf.String(), "Restored")
}

func TestSnapshotConfigFile_EmptyPathIsAbsent(t *testing.T) {
	t.Parallel()

	data, mode, existed, err := snapshotConfigFile("")
	require.NoError(t, err)
	assert.Nil(t, data)
	assert.Zero(t, mode)
	assert.False(t, existed)
}

func TestSnapshotConfigFile_ReturnsReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, _, _, err := snapshotConfigFile(dir)
	require.Error(t, err)
}

func TestRestoreConfigFile_EmptyPathIsSuccess(t *testing.T) {
	t.Parallel()

	assert.True(t, restoreConfigFile(io.Discard, "", []byte("orig"), 0o600, true))
}

func TestWriteBackupFile_CreatesTempBackupWithoutClobbering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "Pulumi.dev.yaml")
	require.NoError(t, os.WriteFile(path, []byte("current"), 0o600))
	require.NoError(t, os.WriteFile(path+".bak", []byte("existing-backup"), 0o600))

	bakPath, err := writeBackupFile(path, []byte("new-backup"), 0o600)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(bakPath, path+".bak."), bakPath)

	gotBak, err := os.ReadFile(path + ".bak")
	require.NoError(t, err)
	assert.Equal(t, []byte("existing-backup"), gotBak)

	gotBackup, err := os.ReadFile(bakPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("new-backup"), gotBackup)
}

func TestWriteBackupFile_ReturnsCreateTempError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "Pulumi.dev.yaml")

	bakPath, err := writeBackupFile(path, []byte("new-backup"), 0o600)
	require.Error(t, err)
	assert.Empty(t, bakPath)
}

func TestStackConfigPath_ReturnsDetectError(t *testing.T) { //nolint: paralleltest
	t.Chdir(t.TempDir())

	path, err := stackConfigPath("dev")
	require.Error(t, err)
	assert.Empty(t, path)
	assert.Contains(t, err.Error(), "detecting project stack path")
}

func TestLoadProjectStack_RemoteConfigWarnsWhenLocalFileExists(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock\n"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}\n"), 0o600))

	project := &workspace.Project{Name: "proj", Runtime: workspace.NewProjectRuntimeInfo("mock", nil)}
	remotePS := &workspace.ProjectStack{Config: config.Map{}}
	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{IsRemote: true} },
		LoadRemoteF: func(ctx context.Context, p *workspace.Project) (*workspace.ProjectStack, error) {
			assert.Equal(t, project.Name, p.Name)
			return remotePS, nil
		},
	}
	var stdout, stderr strings.Builder
	sink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never})

	got, err := LoadProjectStack(t.Context(), sink, project, stack, "")
	require.NoError(t, err)
	assert.Same(t, remotePS, got)
	assert.Contains(t, stderr.String(), "config file")
	assert.Contains(t, stderr.String(), "will be ignored")
}

func TestLoadAndSaveProjectStack_ExplicitConfigFile(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock\n"), 0o600))
	configFile := filepath.Join(wd, "custom-stack.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("config:\n  proj:key: value\n"), 0o600))

	project := &workspace.Project{Name: "proj", Runtime: workspace.NewProjectRuntimeInfo("mock", nil)}
	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
	}
	var stdout, stderr strings.Builder
	sink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never})

	ps, err := LoadProjectStack(t.Context(), sink, project, stack, configFile)
	require.NoError(t, err)
	gotValue, err := ps.Config[config.MustMakeKey("proj", "key")].Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "value", gotValue)

	ps.Config[config.MustMakeKey("proj", "key")] = config.NewValue("updated")
	require.NoError(t, SaveProjectStack(t.Context(), stack, ps, configFile))
	reloaded, err := workspace.LoadProjectStack(sink, project, configFile)
	require.NoError(t, err)
	gotValue, err = reloaded.Config[config.MustMakeKey("proj", "key")].Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "updated", gotValue)
}

func TestLoadProjectStack_ReturnsDetectError(t *testing.T) { //nolint: paralleltest
	t.Chdir(t.TempDir())
	project := &workspace.Project{Name: "proj"}
	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
	}

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	_, err := LoadProjectStack(t.Context(), sink, project, stack, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not detect project stack path")
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
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.ExecuteContext(t.Context())
}

func runMigrateWithOutput(
	t *testing.T,
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	args []string,
) (string, error) {
	t.Helper()
	cmd := newStackMigrateCmd(ws, lm)
	cmd.SetArgs(args)
	var stdout strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), err
}

// shouldForceTargetSecretsRewrite gates the call to CreateSecretsManagerForExistingStack.
// Cloud + default-provider must trigger; everything else must skip (otherwise non-cloud /
// non-default flows re-prompt for passphrase).
func TestShouldForceTargetSecretsRewrite(t *testing.T) {
	t.Parallel()

	cloud := &fakeHTTPStateBackend{MockBackend: &backend.MockBackend{}}
	diy := &backend.MockBackend{}

	tests := []struct {
		name     string
		backend  backend.Backend
		provider string
		want     bool
	}{
		{"cloud + default", cloud, "default", true},
		{"cloud + empty (default)", cloud, "", true},
		{"cloud + passphrase", cloud, "passphrase", false},
		{"cloud + awskms", cloud, "awskms://alias/k", false},
		{"diy + default", diy, "default", false},
		{"diy + passphrase", diy, "passphrase", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, shouldForceTargetSecretsRewrite(tt.backend, tt.provider))
		})
	}
}

func TestReusingSecretsProvider_FallsBackOnDifferentState(t *testing.T) {
	t.Parallel()

	cached := &secrets.MockSecretsManager{
		TypeF:  func() string { return "service" },
		StateF: func() json.RawMessage { return json.RawMessage(`{"url":"https://one"}`) },
	}
	wantState := json.RawMessage(`{"url":"https://two"}`)
	fallbackSM := &secrets.MockSecretsManager{}
	var fallbackCalls int
	fallback := (&secrets.MockProvider{}).Add("service", func(state json.RawMessage) (secrets.Manager, error) {
		fallbackCalls++
		assert.Equal(t, wantState, state)
		return fallbackSM, nil
	})

	got, err := (&reusingSecretsProvider{cached: cached, fallback: fallback}).OfType(t.Context(), "service", wantState)
	require.NoError(t, err)
	assert.Same(t, fallbackSM, got)
	assert.Equal(t, 1, fallbackCalls)
}

func TestReusingSecretsProvider_ReusesCachedManager(t *testing.T) {
	t.Parallel()

	state := json.RawMessage(`{"url":"https://one"}`)
	cached := &secrets.MockSecretsManager{
		TypeF:  func() string { return "service" },
		StateF: func() json.RawMessage { return state },
	}
	fallback := (&secrets.MockProvider{}).Add("service", func(state json.RawMessage) (secrets.Manager, error) {
		t.Fatal("fallback should not be called")
		return nil, nil
	})

	got, err := (&reusingSecretsProvider{cached: cached, fallback: fallback}).OfType(t.Context(), "service", state)
	require.NoError(t, err)
	assert.Same(t, cached, got)
}

func TestStackMigrate_RequiresURLArg(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{}

	err := runMigrate(t, ws, lm, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 1 arg")
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
		"file:///tmp/diy",
		"some-stack",
		"--secrets-provider", "totally-bogus-provider",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown secrets provider")
}

func TestStackMigrate_EarlyErrorPaths(t *testing.T) { //nolint: paralleltest
	sourceURL := "file:///tmp/source"
	project := &workspace.Project{Name: "proj"}

	newSourceBackend := func() *backend.MockBackend {
		return &backend.MockBackend{
			URLF:  func() string { return sourceURL },
			NameF: func() string { return "source" },
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				return &backend.MockStackReference{
					StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
				}, nil
			},
			GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
				return &backend.MockStack{
					RefF:     func() backend.StackReference { return ref },
					BackendF: func() backend.Backend { return nil },
				}, nil
			},
		}
	}
	newTargetBackend := func() *backend.MockBackend {
		return &backend.MockBackend{
			URLF:               func() string { return "https://api.pulumi.com" },
			NameF:              func() string { return "pulumi.com" },
			ValidateStackNameF: func(s string) error { return nil },
			ParseStackReferenceF: func(s string) (backend.StackReference, error) {
				return &backend.MockStackReference{
					StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
				}, nil
			},
			GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
				return nil, nil
			},
		}
	}

	tests := []struct {
		name       string
		ws         pkgWorkspace.Context
		lm         cmdBackend.LoginManager
		targetBE   *backend.MockBackend
		wantSubstr string
	}{
		{
			name: "read project error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return nil, "", errors.New("read boom")
			}},
			lm:         &cmdBackend.MockLoginManager{},
			targetBE:   newTargetBackend(),
			wantSubstr: "read boom",
		},
		{
			name: "source login error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return nil, "", workspace.ErrProjectNotFound
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				return nil, errors.New("login boom")
			}},
			targetBE:   newTargetBackend(),
			wantSubstr: "opening source backend",
		},
		{
			name: "source parse error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "Pulumi.yaml", nil
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				sourceBE := newSourceBackend()
				sourceBE.ParseStackReferenceF = func(s string) (backend.StackReference, error) {
					return nil, errors.New("parse source boom")
				}
				return sourceBE, nil
			}},
			targetBE:   newTargetBackend(),
			wantSubstr: "parsing source stack",
		},
		{
			name: "source lookup error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "Pulumi.yaml", nil
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				sourceBE := newSourceBackend()
				sourceBE.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					return nil, errors.New("lookup boom")
				}
				return sourceBE, nil
			}},
			targetBE:   newTargetBackend(),
			wantSubstr: "looking up source stack",
		},
		{
			name: "source missing",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "Pulumi.yaml", nil
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				sourceBE := newSourceBackend()
				sourceBE.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					return nil, nil
				}
				return sourceBE, nil
			}},
			targetBE:   newTargetBackend(),
			wantSubstr: "not found in backend",
		},
		{
			name: "target validate error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "Pulumi.yaml", nil
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				return newSourceBackend(), nil
			}},
			targetBE: func() *backend.MockBackend {
				be := newTargetBackend()
				be.ValidateStackNameF = func(s string) error { return errors.New("bad target") }
				return be
			}(),
			wantSubstr: "invalid target stack name",
		},
		{
			name: "target parse error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "Pulumi.yaml", nil
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				return newSourceBackend(), nil
			}},
			targetBE: func() *backend.MockBackend {
				be := newTargetBackend()
				be.ParseStackReferenceF = func(s string) (backend.StackReference, error) {
					return nil, errors.New("parse target boom")
				}
				return be
			}(),
			wantSubstr: "parsing target stack",
		},
		{
			name: "target lookup error",
			ws: &pkgWorkspace.MockContext{ReadProjectF: func() (*workspace.Project, string, error) {
				return project, "Pulumi.yaml", nil
			}},
			lm: &cmdBackend.MockLoginManager{LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
			) (backend.Backend, error) {
				return newSourceBackend(), nil
			}},
			targetBE: func() *backend.MockBackend {
				be := newTargetBackend()
				be.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
					return nil, errors.New("target lookup boom")
				}
				return be
			}(),
			wantSubstr: "checking target backend for existing stack",
		},
	}

	oldBE := cmdBackend.BackendInstance
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })
	for _, tt := range tests { //nolint:paralleltest // subtests mutate cmdBackend.BackendInstance.
		t.Run(tt.name, func(t *testing.T) {
			cmdBackend.BackendInstance = tt.targetBE
			err := runMigrate(t, tt.ws, tt.lm, []string{sourceURL, "dev"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantSubstr)
		})
	}
}

func TestStackMigrate_PromptsAndCancelsSameNameMigration(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock\n"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}\n"), 0o600))

	var sourceBE *backend.MockBackend
	sourceStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return b64.NewBase64SecretsManager(), nil
		},
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return sourceStack, nil
		},
	}
	targetBE := &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) { return nil, nil },
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Name: "proj", Runtime: workspace.NewProjectRuntimeInfo("mock", nil)}, wd, nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return sourceBE, nil
		},
	}

	cmd := newStackMigrateCmd(ws, lm)
	cmd.SetArgs([]string{"file:///tmp/source", "dev"})
	cmd.SetIn(strings.NewReader("no\n"))
	var stdout strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	assert.Contains(t, stdout.String(), "will be rewritten with the target's secrets configuration")
	assert.Contains(t, stdout.String(), "Migration cancelled")
}

// Mutates cmdBackend.BackendInstance, so cannot be parallel.
func TestStackMigrate_RejectsSameBackend(t *testing.T) { //nolint: paralleltest
	url := "file:///var/state"

	be := &backend.MockBackend{
		URLF:  func() string { return url },
		NameF: func() string { return "diy" },
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = be
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, loginURL string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				URLF:  func() string { return url },
				NameF: func() string { return "diy" },
			}, nil
		},
	}

	err := runMigrate(t, ws, lm, []string{url, "some-stack"})
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
			return &backend.MockStack{
				RefF: func() backend.StackReference { return ref },
			}, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	err := runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--yes"})
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
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	srcConfigBefore, err := os.ReadFile("Pulumi.dev.yaml")
	require.NoError(t, err)

	err = runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--yes"})
	require.Error(t, err)
	assert.True(t, tgtCreated, "target stack should have been created before the failure")
	assert.True(t, tgtRemoved, "rollback should have removed the partially-created target stack")

	// Same-name migration: source and target share Pulumi.dev.yaml. Confirm rollback restored the
	// file byte-for-byte rather than leaving partial target writes behind.
	srcConfigAfter, err := os.ReadFile("Pulumi.dev.yaml")
	require.NoError(t, err)
	assert.Equal(t, srcConfigBefore, srcConfigAfter,
		"source Pulumi.dev.yaml should be byte-for-byte restored after rollback")
}

// Rename branch of the rollback: source `dev`, target `dev-tgt`. Migration creates
// Pulumi.dev-tgt.yaml mid-flight; rollback must remove that file and leave Pulumi.dev.yaml alone.
func TestStackMigrate_RollsBackTargetConfigOnImportFailure(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	srcPSContent := []byte("config:\n  proj:plain: source-only\n")
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", srcPSContent, 0o600))

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
				StringV:             "dev-tgt",
				NameV:               tokens.MustParseStackName("dev-tgt"),
				FullyQualifiedNameV: "dev-tgt",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
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
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, _ *apitype.UntypedDeployment) error {
			return errors.New("simulated import failure")
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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
		"file:///tmp/source", "dev",
		"--target", "dev-tgt",
		"--yes",
	})
	require.Error(t, err)

	// Source ps untouched.
	srcAfter, err := os.ReadFile("Pulumi.dev.yaml")
	require.NoError(t, err)
	assert.Equal(t, srcPSContent, srcAfter, "source Pulumi.dev.yaml must not be modified")

	// Target ps file was created mid-migration; rollback must remove it.
	_, err = os.Stat("Pulumi.dev-tgt.yaml")
	assert.True(t, os.IsNotExist(err),
		"Pulumi.dev-tgt.yaml should have been removed by the rollback (got err=%v)", err)
}

// Regression for the CreateStack probe race: when CreateStack returns StackAlreadyExistsError,
// the deferred rollback must NOT call RemoveStack on whatever existed at the target ref (it was
// created by another process between our preflight and CreateStack and is not ours to clean up).
func TestStackMigrate_DoesNotRollbackOnAlreadyExistsRace(t *testing.T) { //nolint: paralleltest
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
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return &apitype.UntypedDeployment{Version: 3, Deployment: json.RawMessage(`{}`)}, nil
		},
	}

	// On first GetStack(target): target absent (preflight passes). On the recovery probe after
	// CreateStack fails, return a stack handle simulating the racer's stack -- the guard must NOT
	// adopt and remove it.
	var getStackCalls int
	racerStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev",
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: "dev",
			}
		},
	}
	var removeCalled bool
	targetBE := &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
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
			getStackCalls++
			if getStackCalls == 1 {
				return nil, nil
			}
			return racerStack, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			return nil, &backenderr.StackAlreadyExistsError{StackName: ref.String()}
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			removeCalled = true
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	err := runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--yes"})
	require.Error(t, err)
	assert.False(t, removeCalled,
		"RemoveStack must not be called when CreateStack reports StackAlreadyExistsError")
}

// TestStackMigrate_ReencryptsConfigSecret proves the migration re-encrypts secret config under
// the target's secrets manager. We use a tagging crypter so the test can read the resulting
// Pulumi.<stack>.yaml and observe that the ciphertext was reissued with the target's prefix.
func TestStackMigrate_ReencryptsConfigSecret(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	// Source Pulumi.dev.yaml: one plain config and one secret already encrypted under "src" tag.
	srcYAML := "config:\n" +
		"  proj:plain: hello\n" +
		"  proj:secret:\n" +
		"    secure: src:plaintext-secret\n"
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte(srcYAML), 0o600))

	srcCrypter := prefixCrypter{prefix: "src"}
	tgtCrypter := prefixCrypter{prefix: "tgt"}

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "passphrase" },
		StateF:     func() json.RawMessage { return nil },
		DecrypterF: func() config.Decrypter { return srcCrypter },
		EncrypterF: func() config.Encrypter { return srcCrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{"url":"https://api.pulumi.com"}`) },
		DecrypterF: func() config.Decrypter { return tgtCrypter },
		EncrypterF: func() config.Encrypter { return tgtCrypter },
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
	srcDeployment := &apitype.UntypedDeployment{
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
			return srcDeployment, nil
		},
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

	var imported *apitype.UntypedDeployment
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
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
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			imported = deployment
			return nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	err := runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--yes"})
	require.NoError(t, err)

	// Pulumi.dev.yaml (shared file in same-name migration) should now contain the secret
	// re-encrypted under the target crypter, and not the original source ciphertext.
	post, err := os.ReadFile("Pulumi.dev.yaml")
	require.NoError(t, err)
	postStr := string(post)
	assert.Contains(t, postStr, "secure: tgt:plaintext-secret",
		"target ciphertext should be present after migration: %s", postStr)
	assert.NotContains(t, postStr, "secure: src:plaintext-secret",
		"source ciphertext should have been replaced: %s", postStr)
	assert.Contains(t, postStr, "proj:plain: hello", "plain config should round-trip: %s", postStr)

	// The deployment imported into the target should declare the target's secrets manager,
	// not the source's. Confirms snap.SecretsManager swap + re-serialize ran.
	require.NotNil(t, imported, "ImportDeployment should have been called")
	v3, err := stackUnmarshal(imported)
	require.NoError(t, err)
	assert.Equal(t, "service", v3.SecretsProviders.Type,
		"target's secrets manager should drive the imported deployment")
}

// stackUnmarshal is a tiny helper to avoid importing pkg/v3/resource/stack just for one call.
func stackUnmarshal(d *apitype.UntypedDeployment) (*apitype.DeploymentV3, error) {
	var v3 apitype.DeploymentV3
	if err := json.Unmarshal(d.Deployment, &v3); err != nil {
		return nil, err
	}
	return &v3, nil
}

// TestStackMigrate_ReencryptsStateSecret proves that resource-level secrets in the source
// deployment are re-encrypted under the target's secrets manager. We do this by serializing
// a real snapshot with a `prefixCrypter{src}` so the source deployment carries `src:`-tagged
// ciphertext, then asserting that the deployment imported into the target carries `tgt:`-tagged
// ciphertext (the prefixCrypter the target manager wraps).
func TestStackMigrate_ReencryptsStateSecret(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	ctx := t.Context()

	srcCrypter := prefixCrypter{prefix: "src"}
	tgtCrypter := prefixCrypter{prefix: "tgt"}

	// "test-prefix" is a fake secrets-manager type registered with the custom provider below so
	// DeserializeUntypedDeployment can find a manager for it; the real backend_secrets.DefaultProvider
	// only knows passphrase / service / cloud.
	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "test-prefix" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return srcCrypter },
		EncrypterF: func() config.Encrypter { return srcCrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return tgtCrypter },
		EncrypterF: func() config.Encrypter { return tgtCrypter },
	}

	// Build the source deployment by serializing a snapshot that contains a state secret using
	// srcSM. The resulting JSON contains `src:\"state-plaintext\"` inside the secret's `ciphertext`.
	snap := &deploy.Snapshot{
		SecretsManager: srcSM,
		Resources: []*resource.State{
			{
				URN:  resource.NewURN("dev", "proj", "", resource.RootStackType, "dev"),
				Type: resource.RootStackType,
				Outputs: resource.PropertyMap{
					"stateSecret": resource.MakeSecret(resource.NewProperty("state-plaintext")),
				},
			},
		},
	}
	srcDep, err := stack.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)
	require.Contains(t, string(srcDep.Deployment), `src:\"state-plaintext\"`,
		"sanity check: source deployment should carry the src-tagged ciphertext")

	// Provider that hands out srcSM whenever DeserializeUntypedDeployment asks for "test-prefix".
	customProvider := (&secrets.MockProvider{}).Add(
		"test-prefix",
		func(state json.RawMessage) (secrets.Manager, error) { return srcSM, nil },
	)

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
			return srcDep, nil
		},
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

	var imported *apitype.UntypedDeployment
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
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
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			imported = deployment
			return nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	// Bypass newStackMigrateCmd because the deploymentSecretsProvider field isn't reachable through
	// the cobra closure. Construct the cobra.Command manually so the Run path still has streams.
	smcmd := &stackMigrateCmd{
		secretsProvider:           "default",
		yes:                       true,
		deploymentSecretsProvider: customProvider,
	}
	cobraCmd := &cobra.Command{}
	cobraCmd.SetContext(ctx)
	cobraCmd.SetOut(io.Discard)
	cobraCmd.SetErr(io.Discard)

	err = smcmd.Run(cobraCmd, ws, lm, []string{"file:///tmp/source", "dev"})
	require.NoError(t, err)

	require.NotNil(t, imported, "ImportDeployment should have been called")
	importedJSON := string(imported.Deployment)
	assert.Contains(t, importedJSON, `tgt:\"state-plaintext\"`,
		"resource secret should have been re-encrypted under the target crypter: %s", importedJSON)
	assert.NotContains(t, importedJSON, `src:\"state-plaintext\"`,
		"source ciphertext should have been replaced: %s", importedJSON)
}

// Same-name migration overwrites Pulumi.<stack>.yaml; the command should drop a `.bak` of the
// pre-migration content next to it so the user can recover source-side secrets locally.
func TestStackMigrate_BacksUpSameNameConfigFile(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	srcPSContent := []byte("config:\n  proj:plain: hello\n")
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", srcPSContent, 0o600))

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "passphrase" },
		StateF:     func() json.RawMessage { return nil },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
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
	srcDep := &apitype.UntypedDeployment{
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
			return srcDep, nil
		},
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
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
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
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, _ *apitype.UntypedDeployment) error {
			return nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	err := runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--yes"})
	require.NoError(t, err)

	matches, err := filepath.Glob("Pulumi.dev.yaml.bak.*")
	require.NoError(t, err)
	require.Len(t, matches, 1, "expected one sibling backup with pre-migration source ps content")
	bakBytes, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	assert.Equal(t, srcPSContent, bakBytes, "backup must capture pre-migration bytes")
}

// When a previous migration already left a Pulumi.<stack>.yaml.bak in place, a re-run must NOT
// clobber it. The new backup uses a temp suffix so the original is preserved.
func TestStackMigrate_BackupAvoidsClobberingExistingBak(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config:\n  proj:plain: now\n"), 0o600))
	originalBak := []byte("preserved-original-backup")
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml.bak", originalBak, 0o600))

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "passphrase" },
		StateF:     func() json.RawMessage { return nil },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return srcSM, nil
		},
	}
	srcDep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""}}`),
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}
	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return nil, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, _ *apitype.UntypedDeployment) error {
			return nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	err := runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--yes"})
	require.NoError(t, err)

	// Original .bak preserved.
	got, err := os.ReadFile("Pulumi.dev.yaml.bak")
	require.NoError(t, err)
	assert.Equal(t, originalBak, got, "pre-existing .bak must not be clobbered")

	// New backup uses a temp suffix with the just-overwritten source ps.
	matches, err := filepath.Glob("Pulumi.dev.yaml.bak.*")
	require.NoError(t, err)
	require.Len(t, matches, 1, "expected new backup with temp suffix")
	bakBytes, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	assert.Equal(t, []byte("config:\n  proj:plain: now\n"), bakBytes)
}

// Renaming via --target rewrites the source stack's URNs to the new name in the imported
// deployment, so SaveSnapshot's per-resource URN check passes without requiring --force.
func TestStackMigrate_RewritesURNsOnRename(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	ctx := t.Context()

	srcSM := b64.NewBase64SecretsManager()
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	customProvider := (&secrets.MockProvider{}).Add(
		b64.Type,
		func(state json.RawMessage) (secrets.Manager, error) { return srcSM, nil },
	)

	// Build a real source snapshot with a Stack root + a child resource so we can verify both URN
	// patterns get rewritten (root URN's name uses `<project>-<stack>`, others use the resource name).
	rootURN := resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev")
	childURN := resource.NewURN("dev", "proj", resource.RootStackType, "random:index/randomPet:RandomPet", "pet")
	snap := &deploy.Snapshot{
		SecretsManager: srcSM,
		Resources: []*resource.State{
			{URN: rootURN, Type: resource.RootStackType},
			{URN: childURN, Type: "random:index/randomPet:RandomPet", Parent: rootURN},
		},
	}
	srcDep, err := stack.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)

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
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev-renamed",
				NameV:               tokens.MustParseStackName("dev-renamed"),
				FullyQualifiedNameV: "dev-renamed",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}

	var imported *apitype.UntypedDeployment
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
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
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			imported = deployment
			return nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	// Bypass newStackMigrateCmd so we can inject deploymentSecretsProvider; URN rewrite should
	// make SaveSnapshot's per-resource URN check pass without --force.
	smcmd := &stackMigrateCmd{
		targetStack:               "dev-renamed",
		secretsProvider:           "default",
		yes:                       true,
		deploymentSecretsProvider: customProvider,
	}
	cobraCmd := &cobra.Command{}
	cobraCmd.SetContext(ctx)
	cobraCmd.SetOut(io.Discard)
	cobraCmd.SetErr(io.Discard)

	err = smcmd.Run(cobraCmd, ws, lm, []string{"file:///tmp/source", "dev"})
	require.NoError(t, err)
	require.NotNil(t, imported)
	importedJSON := string(imported.Deployment)
	assert.Contains(t, importedJSON, `urn:pulumi:dev-renamed::proj::pulumi:pulumi:Stack::proj-dev-renamed`,
		"root URN should reference the renamed stack: %s", importedJSON)
	assert.Contains(t, importedJSON, `urn:pulumi:dev-renamed::proj::random:index/randomPet:RandomPet::pet`,
		"child URN should reference the renamed stack: %s", importedJSON)
	assert.NotContains(t, importedJSON, `urn:pulumi:dev::proj::`,
		"no resource URN should still reference the source stack name: %s", importedJSON)
}

// Rename must rewrite URNs in every URN-bearing field of resource.State (Aliases, ViewOf), plus
// ResourceReference URNs nested anywhere inside Inputs / Outputs / ReplacementTrigger property
// trees. This test exercises a state with all of those simultaneously.
func TestStackMigrate_RewritesURNsInAuxiliaryFields(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	ctx := t.Context()

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "test-prefix" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	customProvider := (&secrets.MockProvider{}).Add(
		"test-prefix",
		func(state json.RawMessage) (secrets.Manager, error) { return srcSM, nil },
	)

	rootURN := resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev")
	parentURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Parent", "p")
	siblingURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Sibling", "sib")
	aliasURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Old", "old-name")
	viewURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Owner", "owner")
	depURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Dep", "d")
	propDepURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:PropDep", "pd")
	deletedWithURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Owner", "del")
	replaceWithURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Trigger", "rw")
	providerURN := resource.NewURN("dev", "proj", resource.RootStackType, "pulumi:providers:pkg", "default")
	providerRef, perr := providers.NewReference(providerURN, "providerID")
	require.NoError(t, perr)

	// Inputs holds a ResourceReference; Outputs nests one inside an array+secret; ReplacementTrigger
	// is a top-level ResourceReference. All must be rewritten.
	inputs := resource.PropertyMap{
		"ref": resource.NewProperty(resource.ResourceReference{URN: siblingURN, Type: "pkg:Sibling"}),
	}
	outputs := resource.PropertyMap{
		"refs": resource.NewProperty([]resource.PropertyValue{
			resource.MakeSecret(resource.NewProperty(resource.ResourceReference{URN: siblingURN, Type: "pkg:Sibling"})),
		}),
	}
	replacementTrigger := resource.NewProperty(resource.ResourceReference{URN: siblingURN, Type: "pkg:Sibling"})

	snap := &deploy.Snapshot{
		SecretsManager: srcSM,
		Resources: []*resource.State{
			{URN: rootURN, Type: resource.RootStackType},
			// Provider resource referenced by parent.Provider so the snapshot integrity check
			// resolves the reference to a known provider.
			{URN: providerURN, Type: "pulumi:providers:pkg", Custom: true, ID: "providerID"},
			{
				URN:                  parentURN,
				Type:                 "pkg:Parent",
				Parent:               rootURN,
				Aliases:              []resource.URN{aliasURN},
				ViewOf:               viewURN,
				Inputs:               inputs,
				Outputs:              outputs,
				Dependencies:         []resource.URN{depURN},
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{"k": {propDepURN}},
				DeletedWith:          deletedWithURN,
				ReplaceWith:          []resource.URN{replaceWithURN},
				Provider:             providerRef.String(),
			},
			{
				URN:                siblingURN,
				Type:               "pkg:Sibling",
				Parent:             rootURN,
				ReplacementTrigger: replacementTrigger,
			},
		},
	}
	srcDep, err := stack.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return srcSM, nil
		},
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "dev-renamed",
				NameV:               tokens.MustParseStackName("dev-renamed"),
				FullyQualifiedNameV: "dev-renamed",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}

	var imported *apitype.UntypedDeployment
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return nil, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			imported = deployment
			return nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	// --force lets the integrity check pass even though we didn't materialize every dependency
	// resource above; the test cares only about URN rewriting.
	smcmd := &stackMigrateCmd{
		targetStack:               "dev-renamed",
		secretsProvider:           "default",
		yes:                       true,
		force:                     true,
		deploymentSecretsProvider: customProvider,
	}
	cobraCmd := &cobra.Command{}
	cobraCmd.SetContext(ctx)
	cobraCmd.SetOut(io.Discard)
	cobraCmd.SetErr(io.Discard)

	err = smcmd.Run(cobraCmd, ws, lm, []string{"file:///tmp/source", "dev"})
	require.NoError(t, err)
	require.NotNil(t, imported)
	got := string(imported.Deployment)

	// No URN with old "dev" stack should remain anywhere in the imported deployment.
	assert.NotContains(t, got, `urn:pulumi:dev::proj::`,
		"no URN should still reference the source stack: %s", got)

	// Spot-check that each auxiliary field's URN got renamed.
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Old::old-name`, "alias URN: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Owner::owner`, "viewOf URN: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Sibling::sib`,
		"property-tree ResourceReference URNs (Inputs / Outputs / ReplacementTrigger): %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Dep::d`, "Dependencies URN: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:PropDep::pd`, "PropertyDependencies URN: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Owner::del`, "DeletedWith URN: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Trigger::rw`, "ReplaceWith URN: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pulumi:providers:pkg::default::providerID`,
		"Provider reference URN: %s", got)
}

// --target org/different-proj/stack moves the stack to a new project. URNs in the imported
// deployment must also have their project component rewritten (otherwise they keep referencing
// the source project and stop matching the cloud stack identity).
func TestStackMigrate_RewritesProjectOnTargetWithNewProject(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	ctx := t.Context()

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "test-prefix" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	customProvider := (&secrets.MockProvider{}).Add(
		"test-prefix",
		func(state json.RawMessage) (secrets.Manager, error) { return srcSM, nil },
	)

	rootURN := resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev")
	childURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Child", "c")
	snap := &deploy.Snapshot{
		SecretsManager: srcSM,
		Resources: []*resource.State{
			{URN: rootURN, Type: resource.RootStackType},
			{URN: childURN, Type: "pkg:Child", Parent: rootURN},
		},
	}
	srcDep, err := stack.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
				ProjectV: "proj",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return srcSM, nil
		},
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
				ProjectV: "proj",
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:             "renamed-proj/dev",
				NameV:               tokens.MustParseStackName("dev"),
				ProjectV:            "renamed-proj",
				FullyQualifiedNameV: "renamed-proj/dev",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}

	var imported *apitype.UntypedDeployment
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			// Parse "renamed-proj/dev" -> name=dev, project=renamed-proj.
			parts := strings.Split(s, "/")
			name := parts[len(parts)-1]
			var proj tokens.Name
			if len(parts) >= 2 {
				proj = tokens.Name(parts[len(parts)-2])
			}
			return &backend.MockStackReference{
				StringV:             s,
				NameV:               tokens.MustParseStackName(name),
				ProjectV:            proj,
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
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			imported = deployment
			return nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	// --target keeps stack name "dev" but moves to project "renamed-proj"; URNs must reflect that.
	smcmd := &stackMigrateCmd{
		targetStack:               "renamed-proj/dev",
		secretsProvider:           "default",
		yes:                       true,
		deploymentSecretsProvider: customProvider,
	}
	cobraCmd := &cobra.Command{}
	cobraCmd.SetContext(ctx)
	cobraCmd.SetOut(io.Discard)
	cobraCmd.SetErr(io.Discard)

	err = smcmd.Run(cobraCmd, ws, lm, []string{"file:///tmp/source", "dev"})
	require.NoError(t, err)
	require.NotNil(t, imported)
	got := string(imported.Deployment)

	assert.Contains(t, got, `urn:pulumi:dev::renamed-proj::pulumi:pulumi:Stack::renamed-proj-dev`,
		"root URN should reference the new project: %s", got)
	assert.Contains(t, got, `urn:pulumi:dev::renamed-proj::pkg:Child::c`,
		"child URN should reference the new project: %s", got)
	assert.NotContains(t, got, `::proj::`,
		"no URN should still reference the source project: %s", got)
}

// Positive ErrSaveStackConfig path: b.CreateStack succeeds but the post-create
// SaveProjectStack fails, so CreateStack returns ErrSaveStackConfig. The migrate command must
// adopt the orphaned backend stack and the deferred rollback must remove it.
func TestStackMigrate_RollsBackOnSaveStackConfigError(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test-passphrase-for-test")

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
	}
	srcDep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""}}`),
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	// Target stack uses remote config storage so SaveProjectStack hits SaveRemoteConfig, which
	// we make fail. Combined with --secrets-provider=passphrase (sets EncryptionSalt → needsSave
	// = true), this is a clean way to trigger ErrSaveStackConfig from io.go's CreateStack.
	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF:        func() backend.Backend { return targetBE },
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{IsRemote: true} },
		LoadRemoteF: func(_ context.Context, _ *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
		SaveRemoteF: func(_ context.Context, _ *workspace.ProjectStack) error {
			return errors.New("simulated remote save failure")
		},
	}

	var (
		createCalls int
		probedAfter bool
		removeCalls int
		removedRef  string
	)
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			// First call is the preflight check: target absent. Subsequent calls are the
			// recovery probe after CreateStack returns ErrSaveStackConfig: target present.
			if createCalls == 0 {
				return nil, nil
			}
			probedAfter = true
			return tgtStack, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			createCalls++
			return tgtStack, nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			removeCalls++
			removedRef = s.Ref().String()
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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
		"file:///tmp/source", "dev",
		"--secrets-provider", "passphrase",
		"--yes",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating target stack")
	assert.True(t, probedAfter, "recovery probe should have fired after ErrSaveStackConfig")
	assert.Equal(t, 1, removeCalls, "rollback should call RemoveStack once on the adopted stack")
	assert.Equal(t, "dev", removedRef)
}

func TestStackMigrate_WarnsWhenSaveStackConfigCleanupProbeFails(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test-passphrase-for-test")

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
	}
	srcDep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""}}`),
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF:        func() backend.Backend { return targetBE },
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{IsRemote: true} },
		LoadRemoteF: func(_ context.Context, _ *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
		SaveRemoteF: func(_ context.Context, _ *workspace.ProjectStack) error {
			return errors.New("simulated remote save failure")
		},
	}

	var (
		createCalls int
		removeCalls int
	)
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			if createCalls == 0 {
				return nil, nil
			}
			return nil, errors.New("simulated cleanup probe failure")
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			createCalls++
			return tgtStack, nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			removeCalls++
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	stdout, err := runMigrateWithOutput(t, ws, lm, []string{
		"file:///tmp/source", "dev",
		"--secrets-provider", "passphrase",
		"--yes",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating target stack")
	assert.Contains(t, stdout,
		"Warning: target stack dev may have been created before stack config setup failed, "+
			"but cleanup probe failed: simulated cleanup probe failure.")
	assert.Contains(t, stdout, "Run `pulumi stack rm dev --yes --force` to clean it up manually if it exists.")
	assert.Zero(t, removeCalls, "rollback should not remove an unconfirmed target stack")
}

// Legacy DIY refs return no Project(); the migrate command must fall back to the local
// Pulumi.yaml's project name so foreign-project URNs sharing a stack name aren't rewritten.
func TestStackMigrate_RenameLegacyRefFallsBackToLocalProject(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}"), 0o600))

	ctx := t.Context()

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "test-prefix" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	customProvider := (&secrets.MockProvider{}).Add(
		"test-prefix",
		func(state json.RawMessage) (secrets.Manager, error) { return srcSM, nil },
	)

	// `proj` matches the local Pulumi.yaml; `foreign` is an unrelated project that just happens
	// to share the stack name "dev". Without the fallback fix, the foreign URN would be clobbered.
	rootURN := resource.NewURN("dev", "proj", "", resource.RootStackType, "proj-dev")
	ourURN := resource.NewURN("dev", "proj", resource.RootStackType, "pkg:Mine", "m")
	foreignURN := resource.NewURN("dev", "foreign", resource.RootStackType, "pkg:Foreign", "f")
	snap := &deploy.Snapshot{
		SecretsManager: srcSM,
		Resources: []*resource.State{
			{URN: rootURN, Type: resource.RootStackType},
			{URN: ourURN, Type: "pkg:Mine", Parent: rootURN},
			{URN: foreignURN, Type: "pkg:Foreign", Parent: rootURN},
		},
	}
	srcDep, err := stack.SerializeUntypedDeployment(ctx, snap, nil)
	require.NoError(t, err)

	var sourceBE *backend.MockBackend
	// MockStackReference with no ProjectV: Project() returns ("", false), simulating a legacy
	// DIY ref. The migrate command must fall back to project.Name from ReadProject.
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return srcSM, nil
		},
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev-renamed", NameV: tokens.MustParseStackName("dev-renamed"), FullyQualifiedNameV: "dev-renamed",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}

	var imported *apitype.UntypedDeployment
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return nil, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			imported = deployment
			return nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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

	smcmd := &stackMigrateCmd{
		targetStack:               "dev-renamed",
		secretsProvider:           "default",
		yes:                       true,
		force:                     true,
		deploymentSecretsProvider: customProvider,
	}
	cobraCmd := &cobra.Command{}
	cobraCmd.SetContext(ctx)
	cobraCmd.SetOut(io.Discard)
	cobraCmd.SetErr(io.Discard)

	err = smcmd.Run(cobraCmd, ws, lm, []string{"file:///tmp/source", "dev"})
	require.NoError(t, err)
	require.NotNil(t, imported)
	got := string(imported.Deployment)

	// Local-project URN renamed.
	assert.Contains(t, got, `urn:pulumi:dev-renamed::proj::pkg:Mine::m`,
		"local-project URN should be rewritten: %s", got)
	// Foreign-project URN preserved despite sharing the stack name.
	assert.Contains(t, got, `urn:pulumi:dev::foreign::pkg:Foreign::f`,
		"foreign-project URN must not be rewritten: %s", got)
}

func TestStackMigrate_RefusesLegacyRenameWithoutProject(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)

	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock\n"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config: {}\n"), 0o600))

	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
	}
	srcDep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""}}`),
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	var (
		removeCalls int
		removedRef  string
	)
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev-renamed", NameV: tokens.MustParseStackName("dev-renamed"), FullyQualifiedNameV: "dev-renamed",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return nil, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, _ *apitype.UntypedDeployment) error {
			return nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			removeCalls++
			removedRef = s.Ref().String()
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return sourceBE, nil
		},
	}

	err := runMigrate(t, ws, lm, []string{"file:///tmp/source", "dev", "--target", "dev-renamed", "--yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a source project name")
	assert.Contains(t, err.Error(), "Run this command from a directory containing the source project's Pulumi.yaml")
	assert.Contains(t, err.Error(), "<project>/<stack> or <org>/<project>/<stack>")
	assert.Equal(t, 1, removeCalls, "rollback should remove target stack after late rename failure")
	assert.Equal(t, "dev-renamed", removedRef)
}

// Pre-existing Pulumi.<target>.yaml may carry stale config. Migration must replace, not merge,
// so target-only keys absent from the source are wiped.
func TestStackMigrate_ReplacesPreExistingTargetConfig(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600))
	require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config:\n  proj:fromSource: A\n"), 0o600))
	// Pre-existing target ps with stale config AND stale ESC environment imports.
	require.NoError(t, os.WriteFile("Pulumi.dev-tgt.yaml",
		[]byte("environment:\n  - org/zombie-env\nconfig:\n  proj:stale: zombie\n"), 0o600))

	srcSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "passphrase" },
		StateF:     func() json.RawMessage { return nil },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}
	tgtSM := &secrets.MockSecretsManager{
		TypeF:      func() string { return "service" },
		StateF:     func() json.RawMessage { return json.RawMessage(`{}`) },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
	}

	var sourceBE *backend.MockBackend
	srcStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev", NameV: tokens.MustParseStackName("dev"), FullyQualifiedNameV: "dev",
			}
		},
		BackendF: func() backend.Backend { return sourceBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return srcSM, nil
		},
	}
	srcDep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(`{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""}}`),
	}
	sourceBE = &backend.MockBackend{
		URLF:  func() string { return "file:///tmp/source" },
		NameF: func() string { return "source" },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return srcStack, nil
		},
		ExportDeploymentF: func(ctx context.Context, s backend.Stack) (*apitype.UntypedDeployment, error) {
			return srcDep, nil
		},
	}

	var targetBE *backend.MockBackend
	tgtStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "dev-tgt", NameV: tokens.MustParseStackName("dev-tgt"), FullyQualifiedNameV: "dev-tgt",
			}
		},
		BackendF: func() backend.Backend { return targetBE },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return tgtSM, nil
		},
	}
	targetBE = &backend.MockBackend{
		URLF:               func() string { return "https://api.pulumi.com" },
		NameF:              func() string { return "pulumi.com" },
		ValidateStackNameF: func(s string) error { return nil },
		DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s, NameV: tokens.MustParseStackName(s), FullyQualifiedNameV: tokens.QName(s),
			}, nil
		},
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			return nil, nil
		},
		CreateStackF: func(
			ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			return tgtStack, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, _ *apitype.UntypedDeployment) error {
			return nil
		},
		RemoveStackF: func(ctx context.Context, s backend.Stack, force, removeBackups bool) (bool, error) {
			return false, nil
		},
	}
	oldBE := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = targetBE
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBE })

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
		"file:///tmp/source", "dev", "--target", "dev-tgt", "--yes",
	})
	require.NoError(t, err)

	got, err := os.ReadFile("Pulumi.dev-tgt.yaml")
	require.NoError(t, err)
	gotStr := string(got)
	assert.Contains(t, gotStr, "proj:fromSource: A",
		"target ps should carry source-side config: %s", gotStr)
	assert.NotContains(t, gotStr, "proj:stale",
		"pre-existing target-only config must be wiped, not merged: %s", gotStr)
	assert.NotContains(t, gotStr, "zombie",
		"pre-existing target-only config or env imports must be wiped: %s", gotStr)
}
