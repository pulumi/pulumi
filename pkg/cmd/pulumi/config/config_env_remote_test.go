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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newRemoteConfigEnvCmdForTest builds a configEnvCmd whose stack is remote (ESC-backed). The backing
// environment definition is initialDefYAML; UpdateEnvironmentWithProject records the uploaded YAML
// into *uploaded.
func newRemoteConfigEnvCmdForTest(
	stdin io.Reader,
	stdout io.Writer,
	initialDefYAML string,
	uploaded *[]byte,
	updateErr error,
) *configEnvCmd {
	stackRef := "stack"
	configFile := ""
	env := "testProject/testEnv"
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return []byte(initialDefYAML), "etag", 0, nil
		},
		UpdateEnvironmentWithProjectF: func(
			_ context.Context, _, _, _ string, y []byte, _ string,
		) (apitype.EnvironmentDiagnostics, error) {
			if uploaded != nil {
				*uploaded = y
			}
			return nil, updateErr
		},
	}

	return &configEnvCmd{
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
			NewF: func() (pkgWorkspace.W, error) {
				return &pkgWorkspace.MockW{}, nil
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
						ProjectV:            "testProject",
						FullyQualifiedNameV: "org/testProject/stack",
					}
				},
				OrgNameF: func() string { return "org" },
				BackendF: func() backend.Backend { return be },
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
				},
			}, nil
		},
		loadProjectStack: func(
			_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
		saveProjectStack: func(_ context.Context, _ backend.Stack, _ *workspace.ProjectStack, _ string) error {
			return nil
		},
		stackRef:   &stackRef,
		configFile: &configFile,
	}
}

func TestConfigEnvLsRemote(t *testing.T) {
	t.Parallel()

	const def = `imports:
  - a
  - b
values:
  pulumiConfig:
    testProject:k: v
`
	var stdout bytes.Buffer
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader(""), &stdout, def, nil, nil)
	ls := &configEnvLsCmd{parent: parent, jsonOut: ptr(false)}
	require.NoError(t, ls.run(t.Context(), nil))

	const expected = `ENVIRONMENTS
a
b
`
	assert.Equal(t, expected, cleanStdoutIncludingPrompt(stdout.String()))
}

//nolint:paralleltest // exercises the prompt path with shared survey state
func TestConfigEnvAddRemote(t *testing.T) {
	const def = `imports:
  - a
values:
  pulumiConfig:
    testProject:k: v
`
	var uploaded []byte
	var stdout bytes.Buffer
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader("y"), &stdout, def, &uploaded, nil)
	parent.initArgs()
	add := &configEnvAddCmd{parent: parent, yes: true}
	require.NoError(t, add.run(t.Context(), []string{"c"}))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	require.Equal(t, []any{"a", "c"}, doc["imports"])
	// Unrelated config section is preserved.
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	require.Equal(t, "v", pc["testProject:k"])
}

//nolint:paralleltest // exercises the prompt path with shared survey state
func TestConfigEnvRmConfigFileWritesLocal(t *testing.T) {
	var uploaded []byte
	// The stack is linked to remote config, but an explicit --config-file must route the import edit to
	// the local stack file rather than mutating the backing ESC environment.
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader(""), &bytes.Buffer{}, "imports: []\n", &uploaded, nil)
	parent.ssml = cmdStack.NewStackSecretsManagerLoaderFromEnv()
	configFile := "Pulumi.local.yaml"
	parent.configFile = &configFile
	parent.loadProjectStack = func(
		_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
	) (*workspace.ProjectStack, error) {
		return &workspace.ProjectStack{Environment: workspace.NewEnvironment([]string{"c"})}, nil
	}
	var savedFile string
	parent.saveProjectStack = func(_ context.Context, _ backend.Stack, _ *workspace.ProjectStack, file string) error {
		savedFile = file
		return nil
	}
	parent.initArgs()

	rm := &configEnvRmCmd{parent: parent, yes: true}
	require.NoError(t, rm.run(t.Context(), []string{"c"}))
	require.Equal(t, "Pulumi.local.yaml", savedFile, "--config-file must route the import edit to the local file")
	require.Nil(t, uploaded, "--config-file must not mutate the remote environment")
}

func TestEditRemoteStackEnvironmentRejectsPinned(t *testing.T) {
	t.Parallel()

	configFile := ""
	parent := &configEnvCmd{configFile: &configFile}
	pinned := func(env string) backend.Stack {
		return &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			},
		}
	}
	// The guard is the first statement of editRemoteStackEnvironment, so it rejects before any backend
	// access; a pinned ref via either separator must be refused.
	for _, env := range []string{"testProject/testEnv@5", "testProject/testEnv:5"} {
		err := parent.editRemoteStackEnvironment(t.Context(), true, pinned(env), importOp{})
		require.ErrorContains(t, err, "unpin", env)
	}
}

//nolint:paralleltest // exercises the prompt path with shared survey state
func TestConfigEnvRmRemote(t *testing.T) {
	const def = `imports:
  - a
  - b
values:
  pulumiConfig:
    testProject:k: v
`
	var uploaded []byte
	var stdout bytes.Buffer
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader("y"), &stdout, def, &uploaded, nil)
	parent.initArgs()
	rm := &configEnvRmCmd{parent: parent, yes: true}
	require.NoError(t, rm.run(t.Context(), []string{"a"}))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	require.Equal(t, []any{"b"}, doc["imports"])
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	require.Equal(t, "v", pc["testProject:k"])
}

//nolint:paralleltest // exercises the prompt path with shared survey state
func TestConfigEnvRemotePreservesUnrelatedSections(t *testing.T) {
	const def = `# top comment
imports:
  - a
values:
  environmentVariables:
    FOO: bar # keep me
  pulumiConfig:
    testProject:k: v
`
	t.Run("add", func(t *testing.T) {
		var uploaded []byte
		var stdout bytes.Buffer
		parent := newRemoteConfigEnvCmdForTest(strings.NewReader("y"), &stdout, def, &uploaded, nil)
		parent.initArgs()
		add := &configEnvAddCmd{parent: parent, yes: true}
		require.NoError(t, add.run(t.Context(), []string{"b"}))
		out := string(uploaded)
		require.Contains(t, out, "# top comment")
		require.Contains(t, out, "FOO: bar # keep me")
		require.Contains(t, out, "testProject:k: v")
		require.Contains(t, out, "- b")
	})

	t.Run("rm", func(t *testing.T) {
		var uploaded []byte
		var stdout bytes.Buffer
		parent := newRemoteConfigEnvCmdForTest(strings.NewReader("y"), &stdout, def, &uploaded, nil)
		parent.initArgs()
		rm := &configEnvRmCmd{parent: parent, yes: true}
		require.NoError(t, rm.run(t.Context(), []string{"a"}))
		out := string(uploaded)
		require.Contains(t, out, "# top comment")
		require.Contains(t, out, "FOO: bar # keep me")
		require.Contains(t, out, "testProject:k: v")
	})
}

//nolint:paralleltest // exercises the prompt path with shared survey state
func TestConfigEnvAddRemoteEtagConflictSurfaces(t *testing.T) {
	const def = "imports:\n  - a\n"
	var stdout bytes.Buffer
	// The cloud backend translates esc's 409 into ErrConfigConflict; the editor keys on that sentinel.
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader("y"), &stdout, def, nil,
		fmt.Errorf("updating environment: %w", backend.ErrConfigConflict))
	parent.initArgs()
	add := &configEnvAddCmd{parent: parent, yes: true}
	err := add.run(t.Context(), []string{"b"})
	require.ErrorContains(t, err, "modified concurrently")
}

func TestConfigEnvStatusRemote(t *testing.T) {
	t.Parallel()

	const def = `imports:
  - a
  - b
`
	t.Run("text", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		parent := newRemoteConfigEnvCmdForTest(strings.NewReader(""), &stdout, def, nil, nil)
		parent.initArgs()
		cobraCmd := newConfigEnvCmd(parent.ws, parent.stackRef, parent.configFile)
		cobraCmd.SetContext(t.Context())
		require.NoError(t, parent.runStatus(cobraCmd, false))
		out := cleanStdoutIncludingPrompt(stdout.String())
		require.Contains(t, out, "testProject/testEnv")
		require.Contains(t, out, "a")
		require.Contains(t, out, "b")
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		parent := newRemoteConfigEnvCmdForTest(strings.NewReader(""), &stdout, def, nil, nil)
		parent.initArgs()
		cobraCmd := newConfigEnvCmd(parent.ws, parent.stackRef, parent.configFile)
		cobraCmd.SetContext(t.Context())
		require.NoError(t, parent.runStatus(cobraCmd, true))

		var got struct {
			Source      string   `json:"source"`
			Environment string   `json:"environment"`
			Imports     []string `json:"imports"`
		}
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, "remote", got.Source)
		assert.Equal(t, "testProject/testEnv", got.Environment)
		assert.Equal(t, []string{"a", "b"}, got.Imports)
	})
}

// TestConfigEnvStatusLocal verifies bare `config env` reports the local config file for a local stack
// rather than printing help. The --config-file path is honored so the reported file matches the one
// the command resolved against.
func TestConfigEnvStatusLocal(t *testing.T) {
	t.Parallel()

	newLocal := func(stdout io.Writer) *configEnvCmd {
		stackRef := "stack"
		configFile := "Pulumi.dev.yaml"
		return &configEnvCmd{
			stdout: stdout,
			ws: &pkgWorkspace.MockContext{
				ReadProjectF: func() (*workspace.Project, string, error) {
					return &workspace.Project{Name: "test"}, "", nil
				},
			},
			requireStack: func(
				_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
				_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
			) (backend.Stack, error) {
				return &backend.MockStack{
					ConfigLocationF: func() backend.StackConfigLocation {
						return backend.StackConfigLocation{IsRemote: false}
					},
				}, nil
			},
			loadProjectStack: func(
				_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
			) (*workspace.ProjectStack, error) {
				return &workspace.ProjectStack{Environment: workspace.NewEnvironment([]string{"review-stacks"})}, nil
			},
			stackRef:   &stackRef,
			configFile: &configFile,
		}
	}

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		parent := newLocal(&stdout)
		parent.initArgs()
		cobraCmd := newConfigEnvCmd(parent.ws, parent.stackRef, parent.configFile)
		cobraCmd.SetContext(t.Context())
		require.NoError(t, parent.runStatus(cobraCmd, false))
		out := cleanStdoutIncludingPrompt(stdout.String())
		require.Contains(t, out, "stored locally")
		require.Contains(t, out, "Pulumi.dev.yaml")
		require.Contains(t, out, "review-stacks")
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var stdout bytes.Buffer
		parent := newLocal(&stdout)
		parent.initArgs()
		cobraCmd := newConfigEnvCmd(parent.ws, parent.stackRef, parent.configFile)
		cobraCmd.SetContext(t.Context())
		require.NoError(t, parent.runStatus(cobraCmd, true))

		var got struct {
			Source     string   `json:"source"`
			ConfigFile string   `json:"configFile"`
			Imports    []string `json:"imports"`
		}
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, "local", got.Source)
		assert.Equal(t, "Pulumi.dev.yaml", got.ConfigFile)
		assert.Equal(t, []string{"review-stacks"}, got.Imports)
	})
}

// TestConfigEnvLsRemoteStructuredImport verifies structured imports (e.g. {env: {merge: false}}) are
// listed by name, matching workspace.Environment.Imports.
func TestConfigEnvLsRemoteStructuredImport(t *testing.T) {
	t.Parallel()

	const def = `imports:
  - a
  - b:
      merge: false
values:
  pulumiConfig: {}
`
	var stdout bytes.Buffer
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader(""), &stdout, def, nil, nil)
	ls := &configEnvLsCmd{parent: parent, jsonOut: ptr(false)}
	require.NoError(t, ls.run(t.Context(), nil))

	out := cleanStdoutIncludingPrompt(stdout.String())
	require.Contains(t, out, "a")
	require.Contains(t, out, "b")
}

// TestConfigEnvRmRemoteDuplicate verifies removing a duplicated import drops the last occurrence,
// matching workspace.Environment.Remove.
//
//nolint:paralleltest // exercises the edit/save path
func TestConfigEnvRmRemoteDuplicate(t *testing.T) {
	const def = "imports:\n  - a\n  - a\n"
	var uploaded []byte
	var stdout bytes.Buffer
	parent := newRemoteConfigEnvCmdForTest(strings.NewReader("y"), &stdout, def, &uploaded, nil)
	parent.initArgs()
	rm := &configEnvRmCmd{parent: parent, yes: true}
	require.NoError(t, rm.run(t.Context(), []string{"a"}))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	require.Equal(t, []any{"a"}, doc["imports"])
}

// TestConfigEnvStatusFallsThroughToHelpOnResolveError verifies bare `config env` preserves its prior
// offline/local contract: when no stack can be resolved (logged out, none selected, non-interactive) it
// prints help and returns no error, rather than surfacing the resolution error to every backend.
func TestConfigEnvStatusFallsThroughToHelpOnResolveError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("backend unavailable")
	stackRef := "stack"
	configFile := ""
	parent := &configEnvCmd{
		stdout: io.Discard,
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				return &workspace.Project{Name: "test"}, "", nil
			},
		},
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return nil, sentinel
		},
		stackRef:   &stackRef,
		configFile: &configFile,
	}
	parent.initArgs()
	cobraCmd := newConfigEnvCmd(parent.ws, parent.stackRef, parent.configFile)
	cobraCmd.SetContext(t.Context())
	cobraCmd.SetOut(io.Discard)

	require.NoError(t, parent.runStatus(cobraCmd, false))
}
