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
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newTestRemoteStackForPin(
	tb testing.TB, eb *backend.MockEnvironmentsBackend, escEnv string,
) *backend.MockStack {
	tb.Helper()
	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
		},
		BackendF: func() backend.Backend { return eb },
	}
	s.OrgNameF = func() string { return "myorg" }
	return s
}

func TestStripEscEnvVersion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "myproject/dev", stripEscEnvVersion("myproject/dev"))
	assert.Equal(t, "myproject/dev", stripEscEnvVersion("myproject/dev@3"))
	assert.Equal(t, "myproject/dev", stripEscEnvVersion("myproject/dev@stable"))
}

func TestEscEnvVersion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", escEnvVersion("myproject/dev"))
	assert.Equal(t, "3", escEnvVersion("myproject/dev@3"))
	assert.Equal(t, "stable", escEnvVersion("myproject/dev@stable"))
}

func TestIsStackPinned(t *testing.T) {
	t.Parallel()

	t.Run("local stack is not pinned", func(t *testing.T) {
		t.Parallel()
		s := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: false}
			},
		}
		assert.False(t, isStackPinned(s))
	})

	t.Run("unpinned remote stack", func(t *testing.T) {
		t.Parallel()
		env := "myproject/dev"
		s := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			},
		}
		assert.False(t, isStackPinned(s))
	})

	t.Run("pinned remote stack", func(t *testing.T) {
		t.Parallel()
		env := "myproject/dev@3"
		s := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			},
		}
		assert.True(t, isStackPinned(s))
	})
}

func TestRejectIfPinned(t *testing.T) {
	t.Parallel()

	t.Run("local stack passes", func(t *testing.T) {
		t.Parallel()
		s := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: false}
			},
		}
		require.NoError(t, rejectIfPinned(s))
	})

	t.Run("unpinned remote stack passes", func(t *testing.T) {
		t.Parallel()
		env := "myproject/dev"
		s := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			},
		}
		require.NoError(t, rejectIfPinned(s))
	})

	t.Run("pinned remote stack returns error", func(t *testing.T) {
		t.Parallel()
		env := "myproject/dev@3"
		s := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
			},
		}
		err := rejectIfPinned(s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pinned to version 3")
		assert.Contains(t, err.Error(), "pulumi config env pin latest")
	})
}

func TestConfigPin_ToRevision(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForPin(t, eb, "myproject/dev")

	var savedEnvRef string
	var stdout bytes.Buffer

	cmd := &configPinCmd{
		stdout: &stdout,
		getEnvironment: func(_ context.Context, _ backend.EnvironmentsBackend, _, _, _, version string) error {
			assert.Equal(t, "3", version)
			return nil
		},
		loadRemoteConfig: func(_ context.Context, _ backend.Stack, _ *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{
				Environment: workspace.NewEnvironment([]string{"myproject/dev"}),
			}, nil
		},
		saveRemoteConfig: func(_ context.Context, _ backend.Stack, ps *workspace.ProjectStack) error {
			imports := ps.Environment.Imports()
			require.Len(t, imports, 1)
			savedEnvRef = imports[0]
			return nil
		},
	}

	err := cmd.pinStack(context.Background(), stack, &workspace.Project{}, "myproject/dev", "3")
	require.NoError(t, err)
	assert.Equal(t, "myproject/dev@3", savedEnvRef)
	assert.Contains(t, stdout.String(), "Pinned config to version 3")
}

func TestConfigPin_ToTag(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForPin(t, eb, "myproject/dev")

	var savedEnvRef string
	var stdout bytes.Buffer

	cmd := &configPinCmd{
		stdout: &stdout,
		getEnvironment: func(_ context.Context, _ backend.EnvironmentsBackend, _, _, _, version string) error {
			assert.Equal(t, "stable", version)
			return nil
		},
		loadRemoteConfig: func(_ context.Context, _ backend.Stack, _ *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{
				Environment: workspace.NewEnvironment([]string{"myproject/dev"}),
			}, nil
		},
		saveRemoteConfig: func(_ context.Context, _ backend.Stack, ps *workspace.ProjectStack) error {
			imports := ps.Environment.Imports()
			require.Len(t, imports, 1)
			savedEnvRef = imports[0]
			return nil
		},
	}

	err := cmd.pinStack(context.Background(), stack, &workspace.Project{}, "myproject/dev", "stable")
	require.NoError(t, err)
	assert.Equal(t, "myproject/dev@stable", savedEnvRef)
	assert.Contains(t, stdout.String(), `Pinned config to tag "stable"`)
}

func TestConfigPin_UnpinWithLatest(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForPin(t, eb, "myproject/dev@3")

	var savedEnvRef string
	var stdout bytes.Buffer

	cmd := &configPinCmd{
		stdout: &stdout,
		loadRemoteConfig: func(_ context.Context, _ backend.Stack, _ *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{
				Environment: workspace.NewEnvironment([]string{"myproject/dev@3"}),
			}, nil
		},
		saveRemoteConfig: func(_ context.Context, _ backend.Stack, ps *workspace.ProjectStack) error {
			imports := ps.Environment.Imports()
			require.Len(t, imports, 1)
			savedEnvRef = imports[0]
			return nil
		},
	}

	err := cmd.pinStack(context.Background(), stack, &workspace.Project{}, "myproject/dev@3", "latest")
	require.NoError(t, err)
	assert.Equal(t, "myproject/dev", savedEnvRef)
	assert.Contains(t, stdout.String(), "Unpinned")
}

func TestConfigPin_RejectNotFoundVersion(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForPin(t, eb, "myproject/dev")

	cmd := &configPinCmd{
		stdout: io.Discard,
		getEnvironment: func(_ context.Context, _ backend.EnvironmentsBackend, _, _, _, _ string) error {
			return &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"}
		},
	}

	err := cmd.pinStack(context.Background(), stack, &workspace.Project{}, "myproject/dev", "999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `version "999" not found`)
}

func TestConfigPin_RejectDeletedTag(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForPin(t, eb, "myproject/dev")

	cmd := &configPinCmd{
		stdout: io.Discard,
		getEnvironment: func(_ context.Context, _ backend.EnvironmentsBackend, _, _, _, _ string) error {
			return &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "tag not found"}
		},
	}

	err := cmd.pinStack(context.Background(), stack, &workspace.Project{}, "myproject/dev", "deleted-tag")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `version "deleted-tag" not found`)
}

func TestConfigPin_LocalStackNoOp(t *testing.T) {
	t.Parallel()

	env := "myproject/dev"
	localStack := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: false, EscEnv: &env}
		},
	}

	assert.False(t, isStackPinned(localStack))
}

func TestConfigPin_ValidationError(t *testing.T) {
	t.Parallel()

	eb := &backend.MockEnvironmentsBackend{}
	stack := newTestRemoteStackForPin(t, eb, "myproject/dev")

	cmd := &configPinCmd{
		stdout: io.Discard,
		getEnvironment: func(_ context.Context, _ backend.EnvironmentsBackend, _, _, _, _ string) error {
			return errors.New("network error")
		},
	}

	err := cmd.pinStack(context.Background(), stack, &workspace.Project{}, "myproject/dev", "3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validating version")
}
