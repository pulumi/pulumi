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

const checkoutFQN = "org/proj/dev"

// checkoutTestStack builds a remote-config MockStack named dev whose environment backend returns the
// given definition, etag, and revision.
func checkoutTestStack(escEnv, def, etag string, revision int) *backend.MockStack {
	env := escEnv
	be := &backend.MockEnvironmentsBackend{
		GetEnvironmentF: func(_ context.Context, _, _, _, _ string, _ bool) ([]byte, string, int, error) {
			return []byte(def), etag, revision, nil
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

// newCheckoutCmd wires a checkout command against the given stack and a settings-backed workspace, in a
// temp project directory. It returns the command, the shared settings, and the stdout buffer.
func newCheckoutCmd(t *testing.T, stack backend.Stack) (*configEnvCheckoutCmd, *pkgWorkspace.Settings, *bytes.Buffer) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)

	settings := &pkgWorkspace.Settings{}
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
		stdout:     &stdout,
		diags:      diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		ws:         ws,
		stackRef:   &stackRef,
		configFile: &configFile,
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return stack, nil
		},
		// interactive defaults to false: no secrets-provider prompt.
	}
	return &configEnvCheckoutCmd{parent: parent}, settings, &stdout
}

//nolint:paralleltest // mutates the working directory via t.Chdir
func TestConfigEnvCheckout(t *testing.T) {
	ctx := t.Context()

	const def = "imports:\n  - proj/base\nvalues:\n  pulumiConfig:\n    proj:foo: bar\n"

	t.Run("materializes working copy and records marker", func(t *testing.T) {
		cmd, settings, _ := newCheckoutCmd(t, checkoutTestStack("proj/env", def, "etag-1", 7))
		require.NoError(t, cmd.run(ctx))

		path, err := cmdStack.WorkingCopyPath(tokens.QName("dev"))
		require.NoError(t, err)
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(contents), "Local working copy created by", "banner is written")
		assert.Contains(t, string(contents), "proj:foo", "config is materialized")

		marker, ok := settings.Checkouts[checkoutFQN]
		require.True(t, ok, "marker is recorded")
		assert.Equal(t, "proj/env", marker.EnvRef)
		assert.Equal(t, "etag-1", marker.Etag)
		assert.Equal(t, 7, marker.Revision)
		assert.Equal(t, []string{"proj/base"}, marker.Imports)
		assert.NotEmpty(t, marker.ContentHash)
	})

	t.Run("refuses when already checked out", func(t *testing.T) {
		cmd, settings, _ := newCheckoutCmd(t, checkoutTestStack("proj/env", def, "etag-1", 7))
		settings.Checkouts = map[string]pkgWorkspace.Checkout{checkoutFQN: {EnvRef: "proj/env"}}
		err := cmd.run(ctx)
		require.ErrorContains(t, err, "already checked out")
	})

	t.Run("refuses when pinned", func(t *testing.T) {
		cmd, _, _ := newCheckoutCmd(t, checkoutTestStack("proj/env@5", def, "etag-1", 7))
		err := cmd.run(ctx)
		require.ErrorContains(t, err, "pinned")
	})

	t.Run("refuses on a local-config stack", func(t *testing.T) {
		s := &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
			},
			ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
		}
		cmd, _, _ := newCheckoutCmd(t, s)
		err := cmd.run(ctx)
		require.ErrorContains(t, err, "does not use remote configuration")
	})

	t.Run("refuses to clobber a stray working copy", func(t *testing.T) {
		cmd, _, _ := newCheckoutCmd(t, checkoutTestStack("proj/env", def, "etag-1", 7))
		path, err := cmdStack.WorkingCopyPath(tokens.QName("dev"))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte("stray\n"), 0o600))
		err = cmd.run(ctx)
		require.ErrorContains(t, err, "not checked out on this machine")
	})

	t.Run("refuses environments with non-pulumiConfig values", func(t *testing.T) {
		const withVars = "values:\n  pulumiConfig:\n    proj:foo: bar\n  environmentVariables:\n    FOO: bar\n"
		cmd, _, _ := newCheckoutCmd(t, checkoutTestStack("proj/env", withVars, "etag-1", 7))
		err := cmd.run(ctx)
		require.ErrorContains(t, err, "cannot be represented in a local stack file")
	})
}
