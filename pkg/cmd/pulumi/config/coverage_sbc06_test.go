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
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestConfigEditAndConsoleCommandConstruction(t *testing.T) {
	t.Parallel()
	ws := &pkgWorkspace.MockContext{}
	ref, configFile := "", ""

	edit := newConfigEditCmd(ws, &ref, &configFile)
	require.Equal(t, "edit", edit.Use)
	require.True(t, edit.Hidden)
	require.NotNil(t, edit.Flags().Lookup("editor"))
	require.NotNil(t, edit.Flags().Lookup("show-secrets"))

	console := newConfigEnvConsoleCmd(ws, &ref, &configFile)
	require.Equal(t, "console", console.Use)
	require.True(t, console.Hidden)
}

// emptyURLConsoleBackend implements consoleURLProvider but returns no URL, exercising the empty-URL guard.
type emptyURLConsoleBackend struct{ backend.MockBackend }

func (emptyURLConsoleBackend) CloudConsoleURL(...string) string { return "" }

func remoteStackWithEscEnv(escEnv *string, be backend.Backend) *backend.MockStack {
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: escEnv}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}
}

func TestConfigEnvConsoleErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("no linked environment", func(t *testing.T) {
		t.Parallel()
		cmd := &configEnvConsoleCmd{stdout: &bytes.Buffer{}}
		err := cmd.runWithStack(remoteStackWithEscEnv(nil, &backend.MockBackend{}), "")
		require.ErrorContains(t, err, "no linked environment")
	})

	t.Run("malformed environment reference", func(t *testing.T) {
		t.Parallel()
		bad := "noslash"
		cmd := &configEnvConsoleCmd{stdout: &bytes.Buffer{}}
		err := cmd.runWithStack(remoteStackWithEscEnv(&bad, &backend.MockBackend{}), "")
		require.ErrorContains(t, err, "malformed environment reference")
	})

	t.Run("empty console URL", func(t *testing.T) {
		t.Parallel()
		env := "proj/env"
		cmd := &configEnvConsoleCmd{stdout: &bytes.Buffer{}}
		err := cmd.runWithStack(remoteStackWithEscEnv(&env, &emptyURLConsoleBackend{}), "")
		require.ErrorContains(t, err, "did not provide a console URL")
	})
}

// TestBindBackendURLUnsetsWhenAbsent covers the restore branch where PULUMI_BACKEND_URL was unset
// before binding (the existing edit test exercises only the restore-previous-value branch).
//
//nolint:paralleltest // mutates process environment
func TestBindBackendURLUnsetsWhenAbsent(t *testing.T) {
	const key = "PULUMI_BACKEND_URL"
	prev, had := os.LookupEnv(key)
	t.Cleanup(func() {
		if had {
			//nolint:usetesting // conditional restore in cleanup; t.Setenv cannot unset
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})

	require.NoError(t, os.Unsetenv(key))
	restore := bindBackendURL("new")
	require.Equal(t, "new", os.Getenv(key))
	restore()
	_, stillSet := os.LookupEnv(key)
	require.False(t, stillSet)
}
