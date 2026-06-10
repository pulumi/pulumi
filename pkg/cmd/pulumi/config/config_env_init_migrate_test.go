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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// migrateCapture records what the migration produced.
type migrateCapture struct {
	createdYAML []byte
	updatedYAML []byte
	linkedPS    *workspace.ProjectStack
	savedLocal  *workspace.ProjectStack
}

// newMigrateCmdForTest builds a configEnvInitCmd over a local (non-remote) stack. existingEnvYAML, if
// non-empty, makes GetEnvironment report an existing environment (merge path); otherwise GetEnvironment
// returns empty (create path). The returned capture records the created/updated env YAML and the link.
func newMigrateCmdForTest(
	stdin io.Reader,
	stdout io.Writer,
	projectStackYAML string,
	existingEnvYAML string,
	cap *migrateCapture,
) *configEnvInitCmd {
	stackRef := "stack"
	configFile := ""
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			// An empty fixture means the environment does not exist yet (create path); model that as a
			// 404 so the create-vs-merge decision is driven by the error, not by empty bytes.
			if existingEnvYAML == "" {
				return nil, "", 0, &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"}
			}
			return []byte(existingEnvYAML), "etag", 0, nil
		},
		CreateEnvironmentF: func(
			_ context.Context, _, _, _ string, y []byte,
		) (apitype.EnvironmentDiagnostics, error) {
			cap.createdYAML = y
			return nil, nil
		},
		UpdateEnvironmentWithProjectF: func(
			_ context.Context, _, _, _ string, y []byte, _ string,
		) (apitype.EnvironmentDiagnostics, error) {
			cap.updatedYAML = y
			return nil, nil
		},
	}

	parent := &configEnvCmd{
		stdin:       stdin,
		stdout:      stdout,
		interactive: true,
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				p, err := workspace.LoadProjectBytes(
					[]byte("name: test\nruntime: yaml"), "Pulumi.yaml", encoding.YAML)
				if err != nil {
					return nil, "", err
				}
				return p, "", nil
			},
		},
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return &backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						StringV:             "org/stack",
						NameV:               tokens.MustParseStackName("stack"),
						ProjectV:            "test",
						FullyQualifiedNameV: "org/test/stack",
					}
				},
				OrgNameF: func() string { return "org" },
				BackendF: func() backend.Backend { return be },
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
				DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
					return b64.NewBase64SecretsManager(), nil
				},
				SaveRemoteF: func(_ context.Context, ps *workspace.ProjectStack) error {
					cap.linkedPS = ps
					return nil
				},
			}, nil
		},
		loadProjectStack: func(
			_ context.Context, d diag.Sink, p *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			return workspace.LoadProjectStackBytes(d, p, []byte(projectStackYAML), "Pulumi.stack.yaml", encoding.YAML)
		},
		saveProjectStack: func(_ context.Context, _ backend.Stack, ps *workspace.ProjectStack, _ string) error {
			cap.savedLocal = ps
			return nil
		},
		stackRef:   &stackRef,
		configFile: &configFile,
	}

	return &configEnvInitCmd{parent: parent, remoteConfig: true, yes: true}
}

func pulumiConfigOf(t *testing.T, y []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(y, &doc))
	values, ok := doc["values"].(map[string]any)
	require.True(t, ok, "values section missing")
	pc, ok := values["pulumiConfig"].(map[string]any)
	require.True(t, ok, "pulumiConfig section missing")
	return pc
}

// TestConfigEnvInitMigrateTypesAndSecrets verifies that plaintext, secret, typed, object, and array
// config values migrate into the env's values.pulumiConfig with native types preserved and secrets
// wrapped as fn::secret.
func TestConfigEnvInitMigrateTypesAndSecrets(t *testing.T) {
	t.Parallel()

	stackYAML := `config:
  test:plain: hello
  test:flag: true
  test:count: 7
  test:ratio: 1.5
  test:obj:
    env: prod
    nested:
      deep: 1
  test:list:
    - a
    - b
  test:password:
    secure: aHVudGVyMg==
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.createdYAML, "expected env created")
	pc := pulumiConfigOf(t, cap.createdYAML)

	assert.Equal(t, "hello", pc["test:plain"])
	assert.Equal(t, true, pc["test:flag"])
	assert.Equal(t, 7, pc["test:count"])
	// Untyped float scalars are not coerced to numbers (config's coerce handles only bool/int), so
	// they migrate as strings — matching the existing non-remote `config env init` behavior.
	assert.Equal(t, "1.5", pc["test:ratio"])
	assert.Equal(t, map[string]any{
		"env":    "prod",
		"nested": map[string]any{"deep": 1},
	}, pc["test:obj"])
	assert.Equal(t, []any{"a", "b"}, pc["test:list"])
	assert.Equal(t, map[string]any{"fn::secret": "hunter2"}, pc["test:password"])
}

// TestConfigEnvInitMigratePreservesAndFiltersImports verifies existing imports are preserved and a
// self-import equal to the target env name is filtered out.
func TestConfigEnvInitMigratePreservesAndFiltersImports(t *testing.T) {
	t.Parallel()

	stackYAML := `environment:
  - other-env
  - test/stack
config:
  test:k: v
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(cap.createdYAML, &doc))
	assert.Equal(t, []any{"other-env"}, doc["imports"])
}

// TestConfigEnvInitMigrateRendersBlockYAML verifies the migrated environment is emitted as block YAML,
// not inline JSON. A local stack whose `environment` is an imports list makes Environment.Definition()
// return JSON; yaml.Unmarshal tags those nodes flow-style, and without normalization the whole env
// re-marshals as {imports: [...], values: {...}}.
func TestConfigEnvInitMigrateRendersBlockYAML(t *testing.T) {
	t.Parallel()

	stackYAML := `environment:
  - other-env
config:
  test:k: v
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.createdYAML, "expected env created")
	assert.NotContains(t, string(cap.createdYAML), "{",
		"migrated environment must be block YAML, not inline JSON")
	assert.Contains(t, string(cap.createdYAML), "imports:\n",
		"imports must render as a block sequence")
}

// TestConfigEnvInitMigratePreservesInlineEnvironmentValues verifies that a local stack whose inline
// `environment` block carries its own imports and `values` (pulumiConfig + environmentVariables) keeps
// all of them after migration, with the stack `config` block overriding a colliding inline key and the
// self-import filtered out.
func TestConfigEnvInitMigratePreservesInlineEnvironmentValues(t *testing.T) {
	t.Parallel()

	stackYAML := `environment:
  imports:
    - shared-env
    - test/stack
  values:
    pulumiConfig:
      test:fromenv: envvalue
      test:override: inline
    environmentVariables:
      FOO: bar
config:
  test:override: stackwins
  test:plain: hi
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.createdYAML, "expected env created")

	pc := pulumiConfigOf(t, cap.createdYAML)
	assert.Equal(t, "envvalue", pc["test:fromenv"], "inline env pulumiConfig value must survive")
	assert.Equal(t, "stackwins", pc["test:override"], "stack config must override a colliding inline value")
	assert.Equal(t, "hi", pc["test:plain"])

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(cap.createdYAML, &doc))
	assert.Equal(t, []any{"shared-env"}, doc["imports"], "inline imports survive, self-import filtered")
	values, ok := doc["values"].(map[string]any)
	require.True(t, ok, "values section missing")
	assert.Equal(t, map[string]any{"FOO": "bar"}, values["environmentVariables"],
		"inline environmentVariables must survive migration")
}

// TestConfigEnvInitMigrateMergesInlineEnvironmentVariables verifies that when the target environment
// already exists, the stack's inline environmentVariables are merged into it alongside pulumiConfig,
// preserving unrelated keys already in the target.
func TestConfigEnvInitMigrateMergesInlineEnvironmentVariables(t *testing.T) {
	t.Parallel()

	existingEnv := `values:
  environmentVariables:
    KEEP: keep
  pulumiConfig:
    test:keep: keep
`
	stackYAML := `environment:
  values:
    environmentVariables:
      ADDED: added
config:
  test:new: new
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, existingEnv, &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.updatedYAML, "expected merge path")
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(cap.updatedYAML, &doc))
	values, ok := doc["values"].(map[string]any)
	require.True(t, ok, "values section missing")
	assert.Equal(t, map[string]any{"KEEP": "keep", "ADDED": "added"}, values["environmentVariables"],
		"inline env var merges in; target's unrelated env var survives")
	pc, ok := values["pulumiConfig"].(map[string]any)
	require.True(t, ok, "pulumiConfig section missing")
	assert.Equal(t, "keep", pc["test:keep"], "target's existing pulumiConfig survives")
	assert.Equal(t, "new", pc["test:new"], "stack config key is added")
}

// TestConfigEnvInitMigrateMergePreservesStructuredImports verifies that a structured inline import
// ({env: {merge: false}}) keeps its options when merged into an existing target environment, rather
// than being flattened to a bare import name.
func TestConfigEnvInitMigrateMergePreservesStructuredImports(t *testing.T) {
	t.Parallel()

	existingEnv := `values:
  pulumiConfig:
    test:existing: x
`
	stackYAML := `environment:
  imports:
    - shared/env:
        merge: false
config:
  test:k: v
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, existingEnv, &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.updatedYAML, "expected merge path")
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(cap.updatedYAML, &doc))
	assert.Equal(t, []any{map[string]any{"shared/env": map[string]any{"merge": false}}}, doc["imports"],
		"structured import must keep its options through the merge path")
}

// TestConfigEnvInitMigrateNestedSecrets verifies that secrets nested inside an object or an array are
// wrapped as fn::secret, not migrated as plaintext — the highest-stakes invariant of the migration.
func TestConfigEnvInitMigrateNestedSecrets(t *testing.T) {
	t.Parallel()

	stackYAML := `config:
  test:obj:
    apiKey:
      secure: aHVudGVyMg==
    plain: ok
  test:list:
    - secure: aHVudGVyMg==
    - safe
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.createdYAML, "expected env created")
	pc := pulumiConfigOf(t, cap.createdYAML)

	obj, ok := pc["test:obj"].(map[string]any)
	require.True(t, ok, "test:obj should be an object")
	assert.Equal(t, map[string]any{"fn::secret": "hunter2"}, obj["apiKey"],
		"a secret nested in an object must wrap as fn::secret, not migrate as plaintext")
	assert.Equal(t, "ok", obj["plain"])

	list, ok := pc["test:list"].([]any)
	require.True(t, ok, "test:list should be an array")
	assert.Equal(t, map[string]any{"fn::secret": "hunter2"}, list[0],
		"a secret in an array must wrap as fn::secret, not migrate as plaintext")
	assert.Equal(t, "safe", list[1])
}

// TestConfigEnvInitMigrateReconcileNoSpuriousWarnings covers the advertised reconcile path: after a
// first run wrote the env but the link failed, re-running migrates identical values into the existing
// env. The merge must not warn about overwriting values that did not change.
//
//nolint:paralleltest // captures stdout warnings deterministically
func TestConfigEnvInitMigrateReconcileNoSpuriousWarnings(t *testing.T) {
	existingEnv := `values:
  pulumiConfig:
    test:k: v
    test:m: w
`
	stackYAML := `config:
  test:k: v
  test:m: w
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, existingEnv, &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.updatedYAML, "expected merge path")
	assert.NotContains(t, stdout.String(), "overwriting",
		"re-migrating identical values must not warn about overwriting")
}

// TestConfigEnvInitMigrateMerge verifies merging into an existing environment preserves unrelated
// sections and warns on overwritten keys.
//
//nolint:paralleltest // captures stdout warnings deterministically
func TestConfigEnvInitMigrateMerge(t *testing.T) {
	existingEnv := `# top comment
values:
  environmentVariables:
    FOO: bar # keep me
  pulumiConfig:
    test:existing: old
`
	stackYAML := `config:
  test:existing: new
  test:added: hi
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, existingEnv, &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.updatedYAML, "expected env updated (merge path)")
	out := string(cap.updatedYAML)
	require.Contains(t, out, "# top comment")
	require.Contains(t, out, "FOO: bar # keep me")

	pc := pulumiConfigOf(t, cap.updatedYAML)
	assert.Equal(t, "new", pc["test:existing"])
	assert.Equal(t, "hi", pc["test:added"])

	assert.Contains(t, stdout.String(), `overwriting existing key "test:existing"`)
}

// TestConfigEnvInitMigrateEmptyExistingEnvMerges verifies that an environment which already exists
// but has an empty definition (GetEnvironment returns empty bytes with no error) is merged into, not
// created — creating it would fail because it already exists.
//
//nolint:paralleltest // shares capture state
func TestConfigEnvInitMigrateEmptyExistingEnvMerges(t *testing.T) {
	var cap migrateCapture
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return nil, "etag", 0, nil // exists, but empty definition
		},
		CreateEnvironmentF: func(
			_ context.Context, _, _, _ string, y []byte,
		) (apitype.EnvironmentDiagnostics, error) {
			cap.createdYAML = y
			return nil, nil
		},
		UpdateEnvironmentWithProjectF: func(
			_ context.Context, _, _, _ string, y []byte, _ string,
		) (apitype.EnvironmentDiagnostics, error) {
			cap.updatedYAML = y
			return nil, nil
		},
	}

	var stdout bytes.Buffer
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, "config:\n  test:k: v\n", "x", &cap)
	cmd.parent.requireStack = func(
		_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
		_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
	) (backend.Stack, error) {
		return &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{
					StringV:             "org/stack",
					NameV:               tokens.MustParseStackName("stack"),
					ProjectV:            "test",
					FullyQualifiedNameV: "org/test/stack",
				}
			},
			OrgNameF:        func() string { return "org" },
			BackendF:        func() backend.Backend { return be },
			ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
			DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
				return b64.NewBase64SecretsManager(), nil
			},
			SaveRemoteF: func(_ context.Context, _ *workspace.ProjectStack) error { return nil },
		}, nil
	}

	require.NoError(t, cmd.runMigrate(t.Context()))
	assert.Nil(t, cap.createdYAML, "must not create an environment that already exists")
	require.NotNil(t, cap.updatedYAML, "must merge into the existing (empty) environment")
}

// TestConfigEnvInitMigrateLinks verifies the stack is linked: SaveRemoteConfig is invoked with
// ps.Config == nil and a single import equal to the target env.
func TestConfigEnvInitMigrateLinks(t *testing.T) {
	t.Parallel()

	stackYAML := `config:
  test:k: v
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	require.NotNil(t, cap.linkedPS, "expected SaveRemoteConfig to be called")
	assert.Nil(t, cap.linkedPS.Config)
	assert.Equal(t, []string{"test/stack"}, cap.linkedPS.Environment.Imports())
}

// TestConfigEnvInitMigrateAlreadyRemote verifies an already-remote stack fails cleanly with no env
// write and no link.
func TestConfigEnvInitMigrateAlreadyRemote(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, "config:\n  test:k: v\n", "", &cap)
	// Make the stack report remote config.
	env := "test/stack"
	cmd.parent.requireStack = func(
		_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
		_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
	) (backend.Stack, error) {
		return &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{NameV: tokens.MustParseStackName("stack")}
			},
			OrgNameF: func() string { return "org" },
			BackendF: func() backend.Backend { return &backend.MockEnvironmentsBackend{} },
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			},
		}, nil
	}

	err := cmd.runMigrate(t.Context())
	require.ErrorContains(t, err, "already uses remote configuration")
	assert.Nil(t, cap.createdYAML)
	assert.Nil(t, cap.updatedYAML)
	assert.Nil(t, cap.linkedPS)
}

// TestConfigEnvInitMigrateLocalKeepsFileByDefault verifies non-interactive without --cleanup-local
// keeps the local config file. t.Chdir forbids t.Parallel.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvInitMigrateLocalKeepsFileByDefault(t *testing.T) {
	stackYAML := "config:\n  test:k: v\n"
	stackPath := setupLocalProject(t)

	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	cmd.parent.interactive = false
	require.NoError(t, cmd.runMigrate(t.Context()))

	assert.FileExists(t, stackPath)
	assert.Contains(t, stdout.String(), "Kept the local stack configuration file")
}

// TestConfigEnvInitMigrateLocalDeletesWithFlag verifies --cleanup-local deletes the local config file.
//
//nolint:paralleltest // t.Chdir requires a non-parallel test
func TestConfigEnvInitMigrateLocalDeletesWithFlag(t *testing.T) {
	stackYAML := "config:\n  test:k: v\n"
	stackPath := setupLocalProject(t)

	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	cmd.parent.interactive = false
	cmd.cleanupLocal = true
	require.NoError(t, cmd.runMigrate(t.Context()))

	assert.NoFileExists(t, stackPath)
	assert.Contains(t, stdout.String(), "Deleted the local stack configuration file")
}

// setupLocalProject chdirs into a temp dir holding Pulumi.yaml and Pulumi.stack.yaml so that
// workspace.DetectProjectStackPath resolves to the latter, and returns its path.
func setupLocalProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: test\nruntime: yaml\n"), 0o600))
	stackPath := filepath.Join(dir, "Pulumi.stack.yaml")
	require.NoError(t, os.WriteFile(stackPath, []byte("config:\n  test:k: v\n"), 0o600))
	t.Chdir(dir)
	return stackPath
}

// TestConfigEnvInitMigrateNoPlaintextLeak verifies the plaintext secret never appears in stdout.
func TestConfigEnvInitMigrateNoPlaintextLeak(t *testing.T) {
	t.Parallel()

	stackYAML := `config:
  test:password:
    secure: aHVudGVyMg==
`
	var stdout bytes.Buffer
	var cap migrateCapture
	cmd := newMigrateCmdForTest(strings.NewReader(""), &stdout, stackYAML, "", &cap)
	require.NoError(t, cmd.runMigrate(t.Context()))

	assert.NotContains(t, stdout.String(), "hunter2")
}
