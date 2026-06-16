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
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func discardSink() diag.Sink {
	return diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
}

func wsWithSettings(s *pkgWorkspace.Settings) pkgWorkspace.Context {
	return &pkgWorkspace.MockContext{
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{SettingsF: func() *pkgWorkspace.Settings { return s }}, nil
		},
	}
}

func remoteCheckoutStack(name string) *backend.MockStack {
	env := "proj/env"
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName(name),
				FullyQualifiedNameV: tokens.QName("org/proj/" + name),
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
	}
}

// setupProjectDir chdirs into a temp dir holding a Pulumi.yaml so DetectProjectStackPath resolves.
func setupProjectDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)
}

//nolint:paralleltest // mutates the working directory via t.Chdir
func TestResolveWorkingCopy(t *testing.T) {
	ctx := t.Context()
	sink := discardSink()
	const fqn = "org/proj/dev"

	t.Run("explicit config file wins without touching markers", func(t *testing.T) {
		s := remoteCheckoutStack("dev")
		ws := wsWithSettings(&pkgWorkspace.Settings{})
		got, err := ResolveWorkingCopy(ctx, ws, sink, s, "Pulumi.custom.yaml", false)
		require.NoError(t, err)
		assert.Equal(t, "Pulumi.custom.yaml", got)
	})

	t.Run("local-config stack returns empty", func(t *testing.T) {
		s := &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
			},
			ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
		}
		ws := wsWithSettings(&pkgWorkspace.Settings{})
		got, err := ResolveWorkingCopy(ctx, ws, sink, s, "", false)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("file present and marker present routes to the working copy", func(t *testing.T) {
		setupProjectDir(t)
		s := remoteCheckoutStack("dev")
		path, err := WorkingCopyPath(s.Ref().Name().Q())
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte("config: {}\n"), 0o600))
		ws := wsWithSettings(&pkgWorkspace.Settings{
			Checkouts: map[string]pkgWorkspace.Checkout{fqn: {EnvRef: "proj/env"}},
		})
		got, err := ResolveWorkingCopy(ctx, ws, sink, s, "", false)
		require.NoError(t, err)
		assert.Equal(t, path, got)
	})

	t.Run("deploying makes the checked-out warning more prominent", func(t *testing.T) {
		setupProjectDir(t)
		s := remoteCheckoutStack("dev")
		path, err := WorkingCopyPath(s.Ref().Name().Q())
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte("config: {}\n"), 0o600))

		warn := func(deploying bool) string {
			var stderr bytes.Buffer
			capturing := diag.DefaultSink(io.Discard, &stderr, diag.FormatOptions{Color: colors.Never})
			ws := wsWithSettings(&pkgWorkspace.Settings{
				Checkouts: map[string]pkgWorkspace.Checkout{fqn: {EnvRef: "proj/env"}},
			})
			_, err := ResolveWorkingCopy(ctx, ws, capturing, s, "", deploying)
			require.NoError(t, err)
			return stderr.String()
		}

		read := warn(false)
		assert.Contains(t, read, "using local stack config")
		assert.NotContains(t, read, "this operation uses")

		deploy := warn(true)
		assert.Contains(t, deploy, "this operation uses the uncommitted local stack config")
	})

	t.Run("file present and marker absent is a hard error", func(t *testing.T) {
		setupProjectDir(t)
		s := remoteCheckoutStack("dev")
		path, err := WorkingCopyPath(s.Ref().Name().Q())
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte("config: {}\n"), 0o600))
		ws := wsWithSettings(&pkgWorkspace.Settings{})
		_, err = ResolveWorkingCopy(ctx, ws, sink, s, "", false)
		require.ErrorContains(t, err, "not checked out on this machine")
	})

	t.Run("file absent and marker present clears the dangling marker", func(t *testing.T) {
		setupProjectDir(t)
		s := remoteCheckoutStack("dev")
		settings := &pkgWorkspace.Settings{
			Checkouts: map[string]pkgWorkspace.Checkout{fqn: {EnvRef: "proj/env"}},
		}
		ws := wsWithSettings(settings)
		got, err := ResolveWorkingCopy(ctx, ws, sink, s, "", false)
		require.NoError(t, err)
		assert.Empty(t, got)
		_, ok := settings.Checkouts[fqn]
		assert.False(t, ok, "dangling marker should be cleared")
	})

	t.Run("file absent and marker absent uses remote", func(t *testing.T) {
		setupProjectDir(t)
		s := remoteCheckoutStack("dev")
		ws := wsWithSettings(&pkgWorkspace.Settings{})
		got, err := ResolveWorkingCopy(ctx, ws, sink, s, "", false)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
