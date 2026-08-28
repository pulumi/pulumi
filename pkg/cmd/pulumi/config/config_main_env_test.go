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

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakeEnvStore is an in-memory stand-in for a single named ESC environment. It bumps the revision and
// rotates the etag on every accepted update, and rejects an update that does not carry the etag from the
// most recent read, exactly as the service does.
type fakeEnvStore struct {
	t *testing.T

	yaml     string
	etag     string
	revision int

	// exists reports whether the environment has been created.
	exists bool

	gets          int
	updates       int
	lastEtagSent  string
	revisionCalls map[string]int
}

func newFakeEnvStore(t *testing.T, yaml string) *fakeEnvStore {
	return &fakeEnvStore{
		t:             t,
		yaml:          yaml,
		etag:          "etag-1",
		revision:      1,
		exists:        true,
		revisionCalls: map[string]int{},
	}
}

func (s *fakeEnvStore) backend() *backend.MockEnvironmentsBackend {
	return &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		GetEnvironmentDefinitionF: func(
			_ context.Context, org, envProject, envName, version string,
		) ([]byte, string, int, error) {
			assert.Equal(s.t, "test-org", org)
			assert.Equal(s.t, "payments", envProject)
			assert.Equal(s.t, "dev", envName)
			if !s.exists {
				return nil, "", 0, fmt.Errorf("%w: payments/dev", backend.ErrEnvironmentNotFound)
			}
			s.gets++
			return []byte(s.yaml), s.etag, s.revision, nil
		},
		UpdateEnvironmentDefinitionF: func(
			_ context.Context, org, envProject, envName string, yaml []byte, etag string,
		) (apitype.EnvironmentDiagnostics, int, error) {
			s.updates++
			s.lastEtagSent = etag
			if etag != s.etag {
				return nil, 0, fmt.Errorf("%w: payments/dev", backend.ErrEnvironmentConflict)
			}
			s.yaml = string(yaml)
			s.revision++
			s.etag = fmt.Sprintf("etag-%d", s.revision)
			return nil, s.revision, nil
		},
		GetEnvironmentRevisionF: func(
			_ context.Context, org, envProject, envName, version string,
		) (int, error) {
			ref := envProject + "/" + envName
			s.revisionCalls[ref]++
			switch ref {
			case "payments/dev":
				return s.revision, nil
			case "payments/base":
				return 2, nil
			}
			return 0, fmt.Errorf("no such environment %v", ref)
		},
	}
}

func mainEnvStack(be backend.Backend) *backend.MockStack {
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "test-org/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
		OrgNameF:        func() string { return "test-org" },
		BackendF:        func() backend.Backend { return be },
	}
}

func mainEnvLoginManager(be backend.Backend) cmdBackend.LoginManager {
	return &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool,
		) (backend.Backend, error) {
			return be, nil
		},
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool,
			colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}
}

func loadStackFile(t *testing.T, project *workspace.Project, text string) *workspace.ProjectStack {
	t.Helper()
	ps, err := workspace.LoadProjectStackBytes(
		cmdutil.Diag(), project, []byte(text), "Pulumi.testStack.yaml", encoding.YAML)
	require.NoError(t, err)
	return ps
}

func newMainEnvSetCmd(ps *workspace.ProjectStack, stdout *bytes.Buffer) *configSetCmd {
	return &configSetCmd{
		Stdout: stdout,
		LoadProjectStack: func(
			context.Context, diag.Sink, *workspace.Project, backend.Stack, string,
		) (*workspace.ProjectStack, error) {
			return ps, nil
		},
	}
}

//
// Precedence (DoD 3).
//

func TestEffectiveStackEnv(t *testing.T) {
	t.Parallel()

	project := &workspace.Project{Name: "testProject"}
	localStack := mainEnvStack(nil)

	t.Run("neither field", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "config:\n  testProject:foo: bar\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		assert.Nil(t, envDef)
		assert.Nil(t, mainEnv)
		assert.Empty(t, warnings)
	})

	t.Run("environment only", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "environment:\n  - payments/base\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		assert.Equal(t, ps.Environment, envDef)
		assert.Nil(t, mainEnv)
		assert.Empty(t, warnings)
	})

	t.Run("mainEnvironment only", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		require.NotNil(t, mainEnv)
		assert.Equal(t, "payments/dev", mainEnv.String())
		assert.JSONEq(t, `{"imports":["payments/dev"]}`, string(envDef.Definition()))
		assert.Empty(t, warnings)
	})

	t.Run("mainEnvironment pinned", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev@4\n")
		envDef, mainEnv, _ := effectiveStackEnv(localStack, ps)
		require.NotNil(t, mainEnv)
		assert.JSONEq(t, `{"imports":["payments/dev@4"]}`, string(envDef.Definition()))
	})

	t.Run("mainEnvironment wins over environment", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\nenvironment:\n  - other/env\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		require.NotNil(t, mainEnv)
		assert.JSONEq(t, `{"imports":["payments/dev"]}`, string(envDef.Definition()))
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "'environment' is ignored")
	})

	t.Run("remote stack config wins over mainEnvironment", func(t *testing.T) {
		t.Parallel()

		remote := mainEnvStack(nil)
		remote.ConfigLocationF = func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true}
		}

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
		envDef, mainEnv, warnings := effectiveStackEnv(remote, ps)
		assert.Nil(t, envDef)
		assert.Nil(t, mainEnv)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "'mainEnvironment' is ignored")
	})
}

//
// Reads (DoD 2 and DoD 8).
//

func TestMainEnvironmentReadResolvesThroughEnvironment(t *testing.T) {
	t.Parallel()

	var openedYAML string
	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		OpenYAMLEnvironmentF: func(
			_ context.Context, org string, yaml []byte, _ time.Duration, _ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			assert.Equal(t, "test-org", org)
			openedYAML = string(yaml)
			return &esc.Environment{Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					// Defined by the main environment itself.
					"testProject:instanceCount": {
						Value: "6",
						Trace: esc.Trace{Def: esc.Range{Environment: "payments/dev"}},
					},
					// Inherited through the main environment's own `imports:`.
					"testProject:logLevel": {
						Value: "info",
						Trace: esc.Trace{Def: esc.Range{Environment: "payments/base"}},
					},
				}),
			}}, nil, nil
		},
		GetEnvironmentRevisionF: func(_ context.Context, _, envProject, envName, _ string) (int, error) {
			if envProject+"/"+envName == "payments/dev" {
				return 8, nil
			}
			return 0, errors.New("unexpected environment")
		},
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	var stderr bytes.Buffer
	cfg, err := getStackConfigurationFromProjectStackWithWriter(
		t.Context(), &stderr, s, project, nil, ps, nil)
	require.NoError(t, err)

	// The main environment is resolved as a single import, so ESC resolves its own imports server-side.
	assert.JSONEq(t, `{"imports":["payments/dev"]}`, openedYAML)
	assert.Equal(t, []string{"payments/dev"}, cfg.EnvironmentImports)

	values, ok := cfg.Environment.Value.(map[string]esc.Value)
	require.True(t, ok)
	assert.Contains(t, values, "testProject:instanceCount")
	assert.Contains(t, values, "testProject:logLevel")

	// DoD 8: the revision the run resolved against is reported.
	assert.Contains(t, stderr.String(), "Config source: payments/dev@8")
}

func TestNonMainEnvironmentReadPrintsNoConfigSource(t *testing.T) {
	t.Parallel()

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		OpenYAMLEnvironmentF: func(
			_ context.Context, _ string, _ []byte, _ time.Duration, _ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return &esc.Environment{Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{"testProject:a": esc.NewValue("b")}),
			}}, nil, nil
		},
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "environment:\n  - payments/base\n")

	var stderr bytes.Buffer
	_, err := getStackConfigurationFromProjectStackWithWriter(t.Context(), &stderr, s, project, nil, ps, nil)
	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), "Config source:")
}

//
// Writes (DoD 4, 5, 6).
//

func TestMainEnvironmentConfigSet(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig:\n    testProject:existing: keep\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	tmpdir := t.TempDir()
	configFile := filepath.Join(tmpdir, "Pulumi.testStack.yaml")

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	ws := &pkgWorkspace.MockContext{}

	require.NoError(t, cmd.Run(t.Context(), ws, []string{"testProject:instanceCount", "6"}, project, s, configFile))
	require.NoError(t, cmd.Run(t.Context(), ws, []string{"testProject:region", "us-west-2"}, project, s, configFile))

	// DoD 4: successive writes produce successive revisions.
	assert.Equal(t, "Updated payments/dev@2\nUpdated payments/dev@3\n", stdout.String())
	assert.Equal(t, 3, store.revision)

	// Untyped values keep `pulumi config set` semantics: "6" stays a string.
	assert.Contains(t, store.yaml, `testProject:instanceCount: "6"`)
	assert.Contains(t, store.yaml, "testProject:region: us-west-2")
	// Existing values are preserved by the read-modify-write.
	assert.Contains(t, store.yaml, "testProject:existing: keep")

	// The local stack file is never written on this path.
	_, err := os.Stat(configFile)
	assert.True(t, os.IsNotExist(err))
}

func TestMainEnvironmentConfigSetTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		typ      string
		value    string
		expected string
	}{
		{"", "6", `testProject:test: "6"`},
		{"string", "6", `testProject:test: "6"`},
		{"int", "6", "testProject:test: 6"},
		{"bool", "true", "testProject:test: true"},
		{"float", "1.5", "testProject:test: 1.5"},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			t.Parallel()

			store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
			s := mainEnvStack(store.backend())
			project := &workspace.Project{Name: "testProject"}
			ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

			var stdout bytes.Buffer
			cmd := newMainEnvSetCmd(ps, &stdout)
			cmd.Type = c.typ
			require.NoError(t, cmd.Run(
				t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:test", c.value},
				project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml")))

			assert.Contains(t, store.yaml, c.expected)
		})
	}
}

func TestMainEnvironmentConfigSetSecret(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	tmpdir := t.TempDir()
	configFile := filepath.Join(tmpdir, "Pulumi.testStack.yaml")

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	cmd.Secret = true
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:token", "hunter2"},
		project, s, configFile))

	// DoD 5: the value is an ESC secret, not a stack-secrets-manager ciphertext.
	assert.Contains(t, store.yaml, "fn::secret")
	assert.Contains(t, store.yaml, "hunter2")
	// No secrets provider was ever configured for the stack, and nothing was written to disk...
	assert.Empty(t, ps.SecretsProvider)
	assert.Empty(t, ps.EncryptionSalt)
	assert.Empty(t, ps.EncryptedKey)
	_, err := os.Stat(configFile)
	assert.True(t, os.IsNotExist(err))
	// ...and the plaintext never reaches the command's output.
	assert.NotContains(t, stdout.String(), "hunter2")
	assert.Equal(t, "Updated payments/dev@2\n", stdout.String())
}

func TestMainEnvironmentConfigSetSendsEtagFromRead(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"},
		project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml")))

	// DoD 6: the write carried exactly the etag the read returned.
	assert.Equal(t, "etag-1", store.lastEtagSent)
	assert.Equal(t, 1, store.gets)
	assert.Equal(t, 1, store.updates)
}

func TestMainEnvironmentConfigSetStaleEtag(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	be := store.backend()
	// Simulate a concurrent writer: the environment moves on between the read and the write.
	get := be.GetEnvironmentDefinitionF
	be.GetEnvironmentDefinitionF = func(
		ctx context.Context, org, envProject, envName, version string,
	) ([]byte, string, int, error) {
		yaml, _, revision, err := get(ctx, org, envProject, envName, version)
		return yaml, "stale-etag", revision, err
	}

	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	before := store.yaml
	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	err := cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"},
		project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))

	require.ErrorContains(t, err, "environment payments/dev changed since it was read")
	assert.Equal(t, before, store.yaml, "the environment must not be overwritten")
	assert.Equal(t, 1, store.revision)
}

func TestMainEnvironmentConfigRemove(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t,
		"values:\n  pulumiConfig:\n    testProject:a: one\n    testProject:b: two\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	mainEnv := activeMainEnvironment(io.Discard, s, ps)
	require.NotNil(t, mainEnv)
	w, err := newMainEnvWriter(s, mainEnv)
	require.NoError(t, err)

	var stdout bytes.Buffer
	revision, removed, err := w.removeKey(t.Context(), &stdout, config.MustMakeKey("testProject", "a"))
	require.NoError(t, err)
	assert.True(t, removed)
	assert.Equal(t, 2, revision)
	assert.NotContains(t, store.yaml, "testProject:a")
	assert.Contains(t, store.yaml, "testProject:b: two")

	// Removing a key that isn't there creates no revision.
	revision, removed, err = w.removeKey(t.Context(), &stdout, config.MustMakeKey("testProject", "missing"))
	require.NoError(t, err)
	assert.False(t, removed)
	assert.Zero(t, revision)
	assert.Equal(t, 2, store.revision)
}

func TestMainEnvironmentConfigRemoveCommandPath(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t,
		"values:\n  pulumiConfig:\n    testProject:a: one\n    testProject:b: two\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
	mainEnv := activeMainEnvironment(io.Discard, s, ps)
	require.NotNil(t, mainEnv)

	var out bytes.Buffer
	require.NoError(t, removeFromMainEnvironment(
		t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "a"), false))
	assert.Equal(t, "Updated payments/dev@2\n", out.String())
	assert.NotContains(t, store.yaml, "testProject:a")

	// Removing a key that is not set reports so and creates no revision.
	out.Reset()
	require.NoError(t, removeFromMainEnvironment(
		t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "missing"), false))
	assert.Equal(t, "Configuration key 'testProject:missing' is not set in payments/dev\n", out.String())
	assert.Equal(t, 2, store.revision)

	// `--path` removals are not supported yet.
	err := removeFromMainEnvironment(
		t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "b.c"), true)
	require.ErrorContains(t, err, "'pulumi config rm --path' is not supported yet")
	assert.Equal(t, 2, store.revision)
}

func TestMainEnvironmentWriteRefusals(t *testing.T) {
	t.Parallel()

	project := &workspace.Project{Name: "testProject"}

	t.Run("pinned version", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, "mainEnvironment: payments/dev@4\n")

		var stdout bytes.Buffer
		cmd := newMainEnvSetCmd(ps, &stdout)
		err := cmd.Run(
			t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"},
			project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))
		require.ErrorContains(t, err, "pins it to version \"4\"")
		assert.Equal(t, 0, store.updates)
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "")
		store.exists = false
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

		var stdout bytes.Buffer
		cmd := newMainEnvSetCmd(ps, &stdout)
		err := cmd.Run(
			t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"},
			project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))
		require.ErrorContains(t, err, "environment payments/dev does not exist")
		assert.Equal(t, 0, store.updates)
	})

	t.Run("--path", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

		var stdout bytes.Buffer
		cmd := newMainEnvSetCmd(ps, &stdout)
		cmd.Path = true
		err := cmd.Run(
			t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a.b", "c"},
			project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))
		require.ErrorContains(t, err, "'pulumi config set --path' is not supported yet")
		assert.Equal(t, 0, store.updates)
	})

	t.Run("unsupported subcommands", func(t *testing.T) {
		t.Parallel()

		mainEnv := &workspace.MainEnvironment{Project: "payments", Name: "dev"}
		for _, name := range []string{"copy", "set-all", "rm-all", "refresh"} {
			err := errMainEnvUnsupported(name, mainEnv)
			assert.ErrorContains(t, err, "'pulumi config "+name+"' is not supported yet")
			assert.ErrorContains(t, err, "mainEnvironment: payments/dev")
		}
	})
}

//
// Attribution (DoD 7).
//

func prepareMainEnvListConfig(
	t *testing.T,
	stackFile string,
	env *esc.Environment,
	store *fakeEnvStore,
) (backend.Stack, *workspace.Project, *workspace.ProjectStack, cmdStack.SecretsManagerLoader) {
	t.Helper()

	secretsManager, _, _, _, _ := getCountingBase64SecretsManager(t.Context(), t, false)
	snapshot := &deploy.Snapshot{SecretsManager: stack.NewBatchingCachingSecretsManager(secretsManager)}

	be := store.backend()
	be.CheckYAMLEnvironmentF = func(
		context.Context, string, []byte,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return env, apitype.EnvironmentDiagnostics{}, nil
	}
	be.OpenYAMLEnvironmentF = func(
		context.Context, string, []byte, time.Duration, map[string]string,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return env, apitype.EnvironmentDiagnostics{}, nil
	}

	s := mainEnvStack(be)
	s.SnapshotF = func(context.Context, secrets.Provider) (*deploy.Snapshot, error) { return snapshot, nil }

	project := &workspace.Project{Name: "testProject"}
	return s, project, loadStackFile(t, project, stackFile), cmdStack.SecretsManagerLoader{FallbackToState: true}
}

func mainEnvAttributionEnvironment() *esc.Environment {
	return &esc.Environment{Properties: map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"testProject:instanceCount": {
				Value: "6",
				Trace: esc.Trace{Def: esc.Range{Environment: "payments/dev", Begin: esc.Pos{Line: 4, Column: 7}}},
			},
			"testProject:logLevel": {
				Value: "info",
				Trace: esc.Trace{Def: esc.Range{Environment: "payments/base"}},
			},
		}),
	}}
}

func TestMainEnvironmentListConfigSourceColumn(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, ps, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))

	expected := strings.TrimSpace(`
KEY                        VALUE  SOURCE
testProject:instanceCount  6      payments/dev@8
testProject:logLevel       info   payments/base@2 (imported)
`)
	assert.Equal(t, expected, strings.TrimSpace(stdout.String()))

	// One revision lookup per distinct environment, not one per value.
	assert.Equal(t, 1, store.revisionCalls["payments/dev"])
	assert.Equal(t, 1, store.revisionCalls["payments/base"])
}

func TestMainEnvironmentListConfigUnmigratedValues(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, ps, ssml := prepareMainEnvListConfig(
		t,
		"mainEnvironment: payments/dev\nconfig:\n  testProject:instanceCount: \"9\"\n",
		mainEnvAttributionEnvironment(),
		store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))

	out := stdout.String()
	// The stack file's value wins, and is attributed as unmigrated.
	assert.Contains(t, out, "testProject:instanceCount  9      Pulumi.testStack.yaml (unmigrated)")
	assert.Contains(t, out, "1 configuration value(s) still set in Pulumi.testStack.yaml override payments/dev")
}

func TestMainEnvironmentListConfigJSON(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, ps, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, true, true, ""))
	assert.Contains(t, stdout.String(), `"source": "payments/dev@8"`)
	assert.Contains(t, stdout.String(), `"source": "payments/base@2 (imported)"`)
}

// TestEnvironmentStackListConfigUnchanged is the explicit regression test for DoD 11: an
// `environment:`-only stack gains neither a SOURCE column nor a `source` JSON field.
func TestEnvironmentStackListConfigUnchanged(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	s, project, ps, ssml := prepareMainEnvListConfig(
		t, "environment:\n  - payments/dev\n", mainEnvAttributionEnvironment(), store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))
	expected := strings.TrimSpace(`
KEY                        VALUE
testProject:instanceCount  6
testProject:logLevel       info
`)
	assert.Equal(t, expected, strings.TrimSpace(stdout.String()))
	assert.NotContains(t, stdout.String(), "Config source:")

	stdout.Reset()
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, true, true, ""))
	assert.NotContains(t, stdout.String(), `"source"`)
	assert.Empty(t, store.revisionCalls)
}

func TestShowSource(t *testing.T) {
	t.Parallel()

	mainEnv := &workspace.MainEnvironment{Project: "payments", Name: "dev"}
	base := esc.Value{
		Value: "debug",
		Trace: esc.Trace{Def: esc.Range{Environment: "payments/base"}},
	}
	pulumiEnv := esc.NewValue(map[string]esc.Value{
		"testProject:logLevel": {
			Value: "info",
			Trace: esc.Trace{
				Def:  esc.Range{Environment: "payments/dev", Begin: esc.Pos{Line: 4, Column: 7}},
				Base: &base,
			},
		},
	})
	key := config.MustMakeKey("testProject", "logLevel")
	index := &sourceIndex{sources: map[config.Key]string{key: "payments/dev@8"}}

	var out bytes.Buffer
	showSource(&out, index, mainEnv, key, pulumiEnv)
	assert.Equal(t, "Source: payments/dev@8\n"+
		"  defined at payments/dev:4:7\n"+
		"  overrides payments/base\n", out.String())
}

// writeStackFile writes a stack file to a temporary directory and returns its path.
func writeStackFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Pulumi.testStack.yaml")
	require.NoError(t, os.WriteFile(path, []byte(text), 0o600))
	return path
}

func TestGetConfigShowSource(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, _, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}

	var out bytes.Buffer
	require.NoError(t, getConfig(
		t.Context(), &out, cmdutil.Diag(), ssml, ws, s,
		config.MustMakeKey("testProject", "logLevel"), false, false, true, true,
		writeStackFile(t, "mainEnvironment: payments/dev\n")))

	assert.Equal(t, "info\nSource: payments/base@2 (imported)\n", out.String())
}

func TestShowSourceRequiresMainEnvironment(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	s, project, _, ssml := prepareMainEnvListConfig(
		t, "environment:\n  - payments/dev\n", mainEnvAttributionEnvironment(), store)
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}

	var out bytes.Buffer
	err := getConfig(
		t.Context(), &out, cmdutil.Diag(), ssml, ws, s,
		config.MustMakeKey("testProject", "logLevel"), false, false, true, true,
		writeStackFile(t, "environment:\n  - payments/dev\n"))
	require.ErrorContains(t, err, "--show-source requires the stack to set 'mainEnvironment'")
}

// TestMainEnvironmentUnsupportedSubcommands is the command-level half of DoD 12: subcommands that have no
// environment write path refuse rather than silently writing the local `config:` block.
func TestMainEnvironmentUnsupportedSubcommands(t *testing.T) {
	t.Parallel()

	const stackFile = "mainEnvironment: payments/dev\n"

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "testProject"}, "", nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{Current: "https://api.pulumi.com"}, nil
		},
	}

	t.Run("set-all", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		be := &backend.MockBackend{
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) { return s, nil },
		}

		configPath := writeStackFile(t, stackFile)
		stackName := "testStack"
		cmd := newConfigSetAllCmd(ws, &stackName, mainEnvLoginManager(be), &mockEncrypterFactory{}, &configPath)
		cmd.SetContext(t.Context())
		require.NoError(t, cmd.PersistentFlags().Set("plaintext", "testProject:key=value"))

		err := cmd.RunE(cmd, []string{})
		require.ErrorContains(t, err, "'pulumi config set-all' is not supported yet")

		// Neither the environment nor the stack file was written.
		assert.Equal(t, 0, store.updates)
		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, stackFile, string(data))
	})

	t.Run("refresh", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		envBackend := store.backend()
		envBackend.GetLatestConfigurationF = func(
			context.Context, backend.Stack,
		) (backend.LatestConfiguration, error) {
			return backend.LatestConfiguration{
				Config: config.Map{config.MustMakeKey("testProject", "key1"): config.NewValue("value1")},
			}, nil
		}
		s := mainEnvStack(envBackend)
		be := &backend.MockBackend{
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) { return s, nil },
			ParseStackReferenceF: func(name string) (backend.StackReference, error) {
				return &backend.MockStackReference{
					NameV:               tokens.MustParseStackName(name),
					FullyQualifiedNameV: tokens.QName("test-org/testProject/" + name),
				}, nil
			},
		}

		configPath := writeStackFile(t, stackFile)
		stackName := "testStack"
		cmd := newConfigRefreshCmd(ws, &stackName, mainEnvLoginManager(be), &configPath)
		cmd.SetContext(t.Context())
		require.NoError(t, cmd.PersistentFlags().Set("force", "true"))

		err := cmd.RunE(cmd, []string{})
		require.ErrorContains(t, err, "'pulumi config refresh' is not supported yet")

		assert.Equal(t, 0, store.updates)
		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, stackFile, string(data))
	})
}

// TestMainEnvironmentCopySourceRefused covers the half of DoD 12 that `config copy` reaches through its
// *source* stack: a migrated stack's values live in ESC, not in its `config:` block, so copying from one
// would quietly write an empty (or stale) destination.
func TestMainEnvironmentCopySourceRefused(t *testing.T) { //nolint:paralleltest // t.Chdir forbids t.Parallel
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.yaml"), []byte("name: testProject\nruntime: nodejs\n"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.source.yaml"), []byte("mainEnvironment: payments/dev\n"), 0o600))
	const destFile = "config:\n  testProject:existing: keep\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.dest.yaml"), []byte(destFile), 0o600))

	store := newFakeEnvStore(t, "values:\n  pulumiConfig:\n    testProject:a: one\n")
	source := mainEnvStack(store.backend())
	source.RefF = func() backend.StackReference {
		return &backend.MockStackReference{
			NameV:               tokens.MustParseStackName("source"),
			FullyQualifiedNameV: "test-org/testProject/source",
		}
	}
	dest := mainEnvStack(store.backend())
	dest.RefF = func() backend.StackReference {
		return &backend.MockStackReference{
			NameV:               tokens.MustParseStackName("dest"),
			FullyQualifiedNameV: "test-org/testProject/dest",
		}
	}

	be := &backend.MockBackend{
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			if ref.Name().String() == "dest" {
				return dest, nil
			}
			return source, nil
		},
		ParseStackReferenceF: func(name string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName(name),
				FullyQualifiedNameV: tokens.QName("test-org/testProject/" + name),
			}, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "testProject"}, filepath.Join(dir, "Pulumi.yaml"), nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{Current: "https://api.pulumi.com"}, nil
		},
	}

	stackName, configFile := "source", ""
	cmd := newConfigCopyCmd(ws, &stackName, mainEnvLoginManager(be), &configFile)
	cmd.SetContext(t.Context())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.PersistentFlags().Set("dest", "dest"))

	err := cmd.RunE(cmd, []string{})
	require.ErrorContains(t, err, "'pulumi config copy' is not supported yet")

	// The destination stack file was not touched.
	data, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.dest.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, destFile, string(data))
}

// TestGetConfigShowSourceJSON asserts `--json --show-source` stays parseable: attribution goes inside the
// JSON object rather than being printed alongside it.
func TestGetConfigShowSourceJSON(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, _, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}

	var out bytes.Buffer
	require.NoError(t, getConfig(
		t.Context(), &out, cmdutil.Diag(), ssml, ws, s,
		config.MustMakeKey("testProject", "instanceCount"), false, true, true, true,
		writeStackFile(t, "mainEnvironment: payments/dev\n")))

	var value configValueJSON
	require.NoError(t, json.Unmarshal(out.Bytes(), &value))
	require.NotNil(t, value.Value)
	assert.Equal(t, "6", *value.Value)
	assert.Equal(t, "payments/dev@8", value.Source)
}

// TestMainEnvironmentAttributionPinnedImport asserts a value inherited from a pinned import is attributed to
// the pinned version rather than to the import's latest revision.
func TestMainEnvironmentAttributionPinnedImport(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	env := &esc.Environment{Properties: map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"testProject:logLevel": {
				Value: "info",
				Trace: esc.Trace{Def: esc.Range{Environment: "payments/base@4"}},
			},
		}),
	}}
	s, project, ps, ssml := prepareMainEnvListConfig(t, "mainEnvironment: payments/dev\n", env, store)

	var requestedVersion string
	be := s.Backend().(*backend.MockEnvironmentsBackend)
	inner := be.GetEnvironmentRevisionF
	be.GetEnvironmentRevisionF = func(
		ctx context.Context, org, envProject, envName, version string,
	) (int, error) {
		if envProject+"/"+envName == "payments/base" {
			requestedVersion = version
			return 4, nil
		}
		return inner(ctx, org, envProject, envName, version)
	}

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))

	assert.Equal(t, "4", requestedVersion)
	assert.Contains(t, stdout.String(), "payments/base@4 (imported)")
}

// TestMainEnvironmentWriteWarnsAboutShadowingLocalValue covers the transitional state where a key is set
// both in the stack file and in the environment: the stack file wins on reads, so a write that did not say
// so would report a value the very next read would not return.
func TestMainEnvironmentWriteWarnsAboutShadowingLocalValue(t *testing.T) {
	t.Parallel()

	const stackFile = "mainEnvironment: payments/dev\nconfig:\n  testProject:a: local\n"
	project := &workspace.Project{Name: "testProject"}

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, stackFile)

		var stdout bytes.Buffer
		c := newMainEnvSetCmd(ps, &stdout)
		require.NoError(t, c.Run(
			t.Context(), nil, []string{"testProject:a", "env"}, project, s, writeStackFile(t, stackFile)))

		assert.Contains(t, stdout.String(), "Updated payments/dev@2")
		assert.Contains(t, stdout.String(),
			"warning: 'testProject:a' is also set in Pulumi.testStack.yaml, which shadows the value just written")
	})

	t.Run("rm", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig:\n    testProject:a: env\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, stackFile)
		mainEnv := activeMainEnvironment(io.Discard, s, ps)
		require.NotNil(t, mainEnv)

		var out bytes.Buffer
		require.NoError(t, removeFromMainEnvironment(
			t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "a"), false))
		assert.Contains(t, out.String(), "Updated payments/dev@2")
		assert.Contains(t, out.String(),
			"warning: 'testProject:a' is still set in Pulumi.testStack.yaml, which takes precedence over payments/dev")
	})
}

// TestMainEnvironmentRemoteConfigWarnsOnWritePath asserts that a stack configured both ways is told its
// `mainEnvironment` is ignored, instead of hitting the pre-existing remote-config refusal with no
// explanation.
func TestMainEnvironmentRemoteConfigWarnsOnWritePath(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	escEnv := "payments/remote"
	s.ConfigLocationF = func() backend.StackConfigLocation {
		return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
	}
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	var stdout, stderr bytes.Buffer
	c := newMainEnvSetCmd(ps, &stdout)
	c.Stderr = &stderr
	err := c.Run(t.Context(), nil, []string{"testProject:a", "b"}, project, s, "")

	require.ErrorContains(t, err, "config set not supported for remote stack config")
	assert.Contains(t, stderr.String(),
		"'mainEnvironment' is ignored because this stack's configuration is stored remotely")
	assert.Equal(t, 0, store.updates)
}

// TestMainEnvironmentConfigEnvSubcommands asserts `pulumi config env ls/add/rm` do not silently operate on
// the `environment:` list that a main environment supersedes.
func TestMainEnvironmentConfigEnvSubcommands(t *testing.T) {
	t.Parallel()

	const stackYAML = "mainEnvironment: payments/dev\nenvironment:\n  - payments/legacy\n"
	env := &esc.Environment{Properties: map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{}),
	}}

	newCmd := func(stdout io.Writer) *configEnvCmd {
		return newConfigEnvCmdForTest(
			strings.NewReader(""), stdout, "name: test\nruntime: yaml", stackYAML, env, nil, nil)
	}

	t.Run("ls", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var listed []string
		require.NoError(t, newCmd(&stdout).listStackEnvironments(
			t.Context(), func(_ io.Writer, imports []string) error {
				listed = imports
				return nil
			}))
		assert.Equal(t, []string{"payments/dev"}, listed)
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		err := newCmd(&stdout).editStackEnvironment(
			t.Context(), "add", false, true, func(*workspace.ProjectStack) error { return nil })
		require.ErrorContains(t, err, "'pulumi config env add' is not supported yet")
	})

	t.Run("rm", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		err := newCmd(&stdout).editStackEnvironment(
			t.Context(), "rm", false, true, func(*workspace.ProjectStack) error { return nil })
		require.ErrorContains(t, err, "'pulumi config env rm' is not supported yet")
	})
}
