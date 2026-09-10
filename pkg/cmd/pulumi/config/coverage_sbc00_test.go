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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// remoteStackWithBackend builds a remote (ESC-backed) MockStack whose EscEnv ref and backing
// EnvironmentsBackend are supplied by the caller, so constructor error paths can be exercised.
func remoteStackWithBackend(env *string, be backend.Backend) *backend.MockStack {
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}
}

func TestNewESCConfigEditorErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	env := "proj/env"

	t.Run("backend without environments support", func(t *testing.T) {
		t.Parallel()
		s := remoteStackWithBackend(&env, &backend.MockBackend{NameF: func() string { return "plain" }})
		_, err := newESCConfigEditor(ctx, s)
		require.ErrorContains(t, err, "does not support environments")
	})

	t.Run("missing linked environment", func(t *testing.T) {
		t.Parallel()
		s := remoteStackWithBackend(nil, &backend.MockEnvironmentsBackend{})
		_, err := newESCConfigEditor(ctx, s)
		require.ErrorContains(t, err, "no linked environment")
	})

	t.Run("malformed environment reference", func(t *testing.T) {
		t.Parallel()
		bad := "noslash"
		s := remoteStackWithBackend(&bad, &backend.MockEnvironmentsBackend{})
		_, err := newESCConfigEditor(ctx, s)
		require.ErrorContains(t, err, "environment reference")
	})

	t.Run("GetEnvironment failure", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
				return nil, "", 0, errors.New("boom")
			},
		}
		s := remoteStackWithBackend(&env, be)
		_, err := newESCConfigEditor(ctx, s)
		require.ErrorContains(t, err, "getting environment definition")
	})

	t.Run("unparseable environment definition", func(t *testing.T) {
		t.Parallel()
		be := &backend.MockEnvironmentsBackend{
			GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
				return []byte("\tnot: [valid"), "etag", 0, nil
			},
		}
		s := remoteStackWithBackend(&env, be)
		_, err := newESCConfigEditor(ctx, s)
		require.ErrorContains(t, err, "unmarshaling environment definition")
	})
}

// newESCEditor builds an escConfigEditor over the given definition YAML, asserting the remote path
// is taken. onUpdate (if non-nil) observes/overrides the upload performed by Save.
func newESCEditor(
	t *testing.T,
	initialYAML string,
	onUpdate func(yaml []byte, etag string) (apitype.EnvironmentDiagnostics, error),
) *escConfigEditor {
	t.Helper()
	s := remoteStackForEditor(t, []byte(initialYAML), "etag", onUpdate)
	editor, err := newConfigEditor(t.Context(), s, &workspace.ProjectStack{}, config.NopEncrypter, "")
	require.NoError(t, err)
	esc, ok := editor.(*escConfigEditor)
	require.True(t, ok)
	return esc
}

func TestESCConfigEditorRemoveNoOps(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("no values node", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "{}\n", nil)
		require.NoError(t, e.Remove(ctx, config.MustMakeKey("testProject", "missing"), false))
	})

	t.Run("absent key", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "values:\n  pulumiConfig:\n    testProject:other: v\n", nil)
		require.NoError(t, e.Remove(ctx, config.MustMakeKey("testProject", "missing"), false))
	})

	t.Run("absent array index path", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "values:\n  pulumiConfig: {}\n", nil)
		require.NoError(t, e.Remove(ctx, config.MustMakeKey("testProject", "names[0]"), true))
	})
}

func TestESCConfigEditorSetInvalidPath(t *testing.T) {
	t.Parallel()
	e := newESCEditor(t, "values:\n  pulumiConfig: {}\n", nil)
	err := e.Set(t.Context(), config.MustMakeKey("testProject", "root["), config.NewValue("v"), true)
	require.ErrorContains(t, err, "invalid configuration key path")
}

// TestESCConfigEditorSaveReturnsDiags verifies that Save fails only when the update returns error
// diagnostics. A successful update can carry non-error diagnostics (warnings), which must not turn
// a write into a failure; an empty severity is treated as an error for backward compatibility.
func TestESCConfigEditorSaveReturnsDiags(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	save := func(t *testing.T, diags apitype.EnvironmentDiagnostics) error {
		t.Helper()
		e := newESCEditor(t, "values:\n  pulumiConfig: {}\n",
			func(_ []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
				return diags, nil
			})
		require.NoError(t, e.Set(ctx, config.MustMakeKey("testProject", "k"), config.NewValue("v"), false))
		return e.Save(ctx)
	}

	t.Run("error diagnostic fails", func(t *testing.T) {
		t.Parallel()
		err := save(t, apitype.EnvironmentDiagnostics{{Summary: "bad value", Severity: apitype.DiagError}})
		require.ErrorContains(t, err, "updating environment")
	})

	t.Run("empty severity is treated as error", func(t *testing.T) {
		t.Parallel()
		err := save(t, apitype.EnvironmentDiagnostics{{Summary: "bad value"}})
		require.ErrorContains(t, err, "updating environment")
	})

	t.Run("warning does not fail", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, save(t, apitype.EnvironmentDiagnostics{{Summary: "heads up", Severity: apitype.DiagWarning}}))
	})
}

// TestConfigSetRemoteNilProjectStack covers the guard that rejects a config mutation against a
// remote stack with no service-side configuration (a nil ProjectStack), which would otherwise panic.
func TestConfigSetRemoteNilProjectStack(t *testing.T) {
	t.Parallel()
	env := "testProject/testStack"
	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
	}
	cmd := &configSetCmd{
		LoadProjectStack: func(
			_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			return nil, nil
		},
	}
	project := &workspace.Project{Name: "testProject"}
	ws := &pkgWorkspace.MockContext{}
	err := cmd.Run(t.Context(), ws, []string{"testProject:k", "v"}, project, s, "")
	require.ErrorContains(t, err, "config env init")
}

// TestConfigSetRemotePlaintext covers the non-secret remote write path of configSetCmd.Run: the
// value is routed through the ESC editor and uploaded as a native scalar.
//
//nolint:paralleltest // exercises the command with shared mock state
func TestConfigSetRemotePlaintext(t *testing.T) {
	ctx := t.Context()

	var uploaded []byte
	s := remoteStackForEditor(t, []byte("values:\n  pulumiConfig: {}\n"), "etag",
		func(y []byte, _ string) (apitype.EnvironmentDiagnostics, error) {
			uploaded = y
			return nil, nil
		})

	cmd := &configSetCmd{
		LoadProjectStack: func(
			_ context.Context, _ diag.Sink, _ *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{Config: config.Map{}}, nil
		},
	}
	project := &workspace.Project{Name: "testProject"}
	ws := &pkgWorkspace.MockContext{}
	require.NoError(t, cmd.Run(ctx, ws, []string{"testProject:key", "value"}, project, s, ""))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(uploaded, &doc))
	pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
	require.Equal(t, "value", pc["testProject:key"])
}
