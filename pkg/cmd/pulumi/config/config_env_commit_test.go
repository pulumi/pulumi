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
	"os"
	"path/filepath"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// commitStack builds a remote-config MockStack named dev. getDef/getEtag/getRevision drive
// GetEnvironment; onUpdate (if non-nil) captures the uploaded YAML.
func commitStack(
	escEnv, getDef, getEtag string, getRevision int,
	onUpdate func(yaml []byte, etag string) (apitype.EnvironmentDiagnostics, error),
) *backend.MockStack {
	env := escEnv
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return []byte(getDef), getEtag, getRevision, nil
		},
		UpdateEnvironmentWithProjectF: func(
			_ context.Context, _, _, _ string, y []byte, etag string,
		) (apitype.EnvironmentDiagnostics, error) {
			if onUpdate != nil {
				return onUpdate(y, etag)
			}
			return nil, nil
		},
	}
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: tokens.QName(checkoutFQN),
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}
}

// newCommitCmd wires a commit command in a temp project dir, writing workingCopy to the working-copy
// path and seeding the given marker. Returns the command, shared settings, the working-copy path.
func newCommitCmd(
	t *testing.T, stack backend.Stack, marker pkgWorkspace.Checkout, workingCopy string,
) (*configEnvCommitCmd, *pkgWorkspace.Settings, string) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)

	path, err := cmdStack.WorkingCopyPath(tokens.QName("dev"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(workingCopy), 0o600))
	marker.FilePath = path

	settings := &pkgWorkspace.Settings{Checkouts: map[string]pkgWorkspace.Checkout{checkoutFQN: marker}}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Name: "proj"}, "", nil
		},
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{SettingsF: func() *pkgWorkspace.Settings { return settings }}, nil
		},
	}
	var stdout bytes.Buffer
	stackRef, configFile := "", ""
	parent := &configEnvCmd{
		stdout:           &stdout,
		diags:            diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		ws:               ws,
		stackRef:         &stackRef,
		configFile:       &configFile,
		loadProjectStack: cmdStack.LoadProjectStack,
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return stack, nil
		},
	}
	return &configEnvCommitCmd{parent: parent}, settings, path
}

// hashOfWorkingCopy loads a working-copy file the way commit does and returns its canonical hash.
func hashOfWorkingCopy(t *testing.T, path string) string {
	t.Helper()
	ps, err := cmdStack.LoadProjectStack(t.Context(),
		diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		&workspace.Project{Name: "proj"}, nil, path)
	require.NoError(t, err)
	h, err := canonicalCheckoutHash(ps)
	require.NoError(t, err)
	return h
}

//nolint:paralleltest // mutates the working directory via t.Chdir
func TestConfigEnvCommit(t *testing.T) {
	ctx := t.Context()

	t.Run("refuses when not checked out", func(t *testing.T) {
		cmd, settings, _ := newCommitCmd(t, commitStack("proj/env", "", "etag", 1, nil),
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag"}, "config:\n  proj:a: b\n")
		delete(settings.Checkouts, checkoutFQN)
		require.ErrorContains(t, cmd.run(ctx), "not checked out")
	})

	t.Run("refuses when pinned after checkout", func(t *testing.T) {
		cmd, _, _ := newCommitCmd(t, commitStack("proj/env@5", "", "etag", 1, nil),
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag"}, "config:\n  proj:a: b\n")
		require.ErrorContains(t, cmd.run(ctx), "pinned")
	})

	t.Run("refuses when relinked after checkout", func(t *testing.T) {
		cmd, _, _ := newCommitCmd(t, commitStack("proj/other", "", "etag", 1, nil),
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag"}, "config:\n  proj:a: b\n")
		require.ErrorContains(t, cmd.run(ctx), "relinked")
	})

	t.Run("no-op when unchanged: cleans up without writing", func(t *testing.T) {
		var uploaded bool
		stack := commitStack("proj/env", "values:\n  pulumiConfig:\n    proj:a: b\n", "etag", 1,
			func([]byte, string) (apitype.EnvironmentDiagnostics, error) { uploaded = true; return nil, nil })
		cmd, settings, path := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag"}, "config:\n  proj:a: b\n")
		// Seed the marker hash to match the (unchanged) working copy.
		marker := settings.Checkouts[checkoutFQN]
		marker.ContentHash = hashOfWorkingCopy(t, path)
		settings.Checkouts[checkoutFQN] = marker

		require.NoError(t, cmd.run(ctx))
		assert.False(t, uploaded, "no-op commit must not write the environment")
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "working copy removed")
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})

	t.Run("no-op cleans up even when relinked", func(t *testing.T) {
		var uploaded bool
		stack := commitStack("proj/other", "values:\n  pulumiConfig:\n    proj:a: b\n", "etag", 1,
			func([]byte, string) (apitype.EnvironmentDiagnostics, error) { uploaded = true; return nil, nil })
		cmd, settings, path := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag"}, "config:\n  proj:a: b\n")
		marker := settings.Checkouts[checkoutFQN]
		marker.ContentHash = hashOfWorkingCopy(t, path)
		settings.Checkouts[checkoutFQN] = marker

		require.NoError(t, cmd.run(ctx), "a no-op commit writes nothing, so pin/relink state is irrelevant")
		assert.False(t, uploaded, "no-op commit must not write the environment")
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})

	t.Run("exact replacement removes env-only keys", func(t *testing.T) {
		var uploaded []byte
		envDef := "values:\n  pulumiConfig:\n    proj:a: original\n    proj:old: gone\n"
		stack := commitStack("proj/env", envDef, "etag", 3,
			func(y []byte, _ string) (apitype.EnvironmentDiagnostics, error) { uploaded = y; return nil, nil })
		cmd, settings, path := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag", Revision: 3, ContentHash: "stale"},
			"config:\n  proj:a: b\n")

		require.NoError(t, cmd.run(ctx))

		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(uploaded, &doc))
		pc := doc["values"].(map[string]any)["pulumiConfig"].(map[string]any)
		assert.Equal(t, "b", pc["proj:a"], "edited key overwritten")
		_, hasOld := pc["proj:old"]
		assert.False(t, hasOld, "key absent from the working copy is removed")
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "working copy removed")
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})

	t.Run("changed imports are written back in order", func(t *testing.T) {
		var uploaded []byte
		envDef := "imports:\n  - proj/a\n  - proj/b\nvalues:\n  pulumiConfig:\n    proj:k: v\n"
		stack := commitStack("proj/env", envDef, "etag", 2,
			func(y []byte, _ string) (apitype.EnvironmentDiagnostics, error) { uploaded = y; return nil, nil })
		cmd, _, _ := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "etag", Imports: []string{"proj/a", "proj/b"}, ContentHash: "stale"},
			"environment:\n  - proj/b\n  - proj/c\nconfig:\n  proj:k: v\n")

		require.NoError(t, cmd.run(ctx))

		var doc struct {
			Imports []string `yaml:"imports"`
		}
		require.NoError(t, yaml.Unmarshal(uploaded, &doc))
		assert.Equal(t, []string{"proj/b", "proj/c"}, doc.Imports, "import list rewritten to the working copy's order")
	})

	t.Run("drift refuses non-interactively without --force-revision", func(t *testing.T) {
		stack := commitStack("proj/env", "values:\n  pulumiConfig:\n    proj:a: x\n", "new-etag", 9, nil)
		cmd, _, _ := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "old-etag", Revision: 3, ContentHash: "stale"},
			"config:\n  proj:a: b\n")
		require.ErrorContains(t, cmd.run(ctx), "--force-revision 9")
	})

	t.Run("force-revision lease passes at the asserted revision", func(t *testing.T) {
		var uploaded []byte
		stack := commitStack("proj/env", "values:\n  pulumiConfig:\n    proj:a: x\n", "new-etag", 9,
			func(y []byte, _ string) (apitype.EnvironmentDiagnostics, error) { uploaded = y; return nil, nil })
		cmd, _, _ := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "old-etag", Revision: 3, ContentHash: "stale"},
			"config:\n  proj:a: b\n")
		cmd.forced = true
		cmd.forceRevision = 9
		require.NoError(t, cmd.run(ctx))
		require.NotNil(t, uploaded, "lease satisfied, environment written")
	})

	t.Run("force-revision lease refuses at a different revision", func(t *testing.T) {
		stack := commitStack("proj/env", "values:\n  pulumiConfig:\n    proj:a: x\n", "new-etag", 9, nil)
		cmd, _, _ := newCommitCmd(t, stack,
			pkgWorkspace.Checkout{EnvRef: "proj/env", Etag: "old-etag", Revision: 3, ContentHash: "stale"},
			"config:\n  proj:a: b\n")
		cmd.forced = true
		cmd.forceRevision = 7
		require.ErrorContains(t, cmd.run(ctx), "now at revision 9, not 7")
	})
}
