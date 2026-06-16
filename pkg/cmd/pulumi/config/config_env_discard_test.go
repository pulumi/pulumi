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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func discardStack() *backend.MockStack {
	env := "proj/env"
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
	}
}

// newDiscardCmd wires a discard command in a temp project dir. If writeFile is true it writes workingCopy
// to the working-copy path. Returns the command, shared settings, and the path.
func newDiscardCmd(
	t *testing.T, marker pkgWorkspace.Checkout, workingCopy string, writeFile, interactive bool,
) (*configEnvDiscardCmd, *pkgWorkspace.Settings, string) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)

	path, err := cmdStack.WorkingCopyPath(tokens.QName("dev"))
	require.NoError(t, err)
	if writeFile {
		require.NoError(t, os.WriteFile(path, []byte(workingCopy), 0o600))
	}
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
	stack := discardStack()
	parent := &configEnvCmd{
		stdout:           &stdout,
		diags:            diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		ws:               ws,
		stackRef:         &stackRef,
		configFile:       &configFile,
		interactive:      interactive,
		loadProjectStack: cmdStack.LoadProjectStack,
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return stack, nil
		},
	}
	return &configEnvDiscardCmd{parent: parent}, settings, path
}

//nolint:paralleltest // mutates the working directory via t.Chdir
func TestConfigEnvDiscard(t *testing.T) {
	ctx := t.Context()
	const wc = "config:\n  proj:a: b\n"

	t.Run("refuses when not checked out", func(t *testing.T) {
		cmd, settings, _ := newDiscardCmd(t, pkgWorkspace.Checkout{EnvRef: "proj/env"}, wc, true, false)
		delete(settings.Checkouts, checkoutFQN)
		require.ErrorContains(t, cmd.run(ctx), "not checked out")
	})

	t.Run("unchanged working copy is removed without confirmation", func(t *testing.T) {
		cmd, settings, path := newDiscardCmd(t, pkgWorkspace.Checkout{EnvRef: "proj/env"}, wc, true, false)
		marker := settings.Checkouts[checkoutFQN]
		marker.ContentHash = hashOfWorkingCopy(t, path)
		settings.Checkouts[checkoutFQN] = marker

		require.NoError(t, cmd.run(ctx)) // non-interactive, no --yes, but no-op needs no confirmation
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "working copy removed")
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})

	t.Run("changed working copy refuses non-interactively without --yes", func(t *testing.T) {
		cmd, _, path := newDiscardCmd(t, pkgWorkspace.Checkout{EnvRef: "proj/env", ContentHash: "stale"}, wc, true, false)
		require.Error(t, cmd.run(ctx))
		_, statErr := os.Stat(path)
		assert.False(t, os.IsNotExist(statErr), "working copy preserved on refusal")
	})

	t.Run("changed working copy is discarded with --yes", func(t *testing.T) {
		marker := pkgWorkspace.Checkout{EnvRef: "proj/env", ContentHash: "stale"}
		cmd, settings, path := newDiscardCmd(t, marker, wc, true, false)
		cmd.yes = true
		require.NoError(t, cmd.run(ctx))
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "working copy removed")
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})

	t.Run("corrupt working copy is still discardable with --yes", func(t *testing.T) {
		marker := pkgWorkspace.Checkout{EnvRef: "proj/env", ContentHash: "stale"}
		cmd, settings, path := newDiscardCmd(t, marker, "\tnot: [valid yaml", true, false)
		cmd.yes = true
		require.NoError(t, cmd.run(ctx))
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "corrupt working copy removed")
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})

	t.Run("missing working copy clears the dangling marker", func(t *testing.T) {
		marker := pkgWorkspace.Checkout{EnvRef: "proj/env", ContentHash: "stale"}
		cmd, settings, _ := newDiscardCmd(t, marker, wc, false, false)
		require.NoError(t, cmd.run(ctx))
		_, ok := settings.Checkouts[checkoutFQN]
		assert.False(t, ok, "marker cleared")
	})
}
