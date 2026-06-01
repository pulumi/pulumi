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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newRollbackCmdForTest builds a configEnvRollbackCmd over a remote stack linked to escEnv in org
// "org". runEnv captures the `pulumi env` args the command would dispatch.
func newRollbackCmdForTest(
	t *testing.T, escEnv, configFile string, runEnv func(context.Context, []string) error,
) *configEnvRollbackCmd {
	t.Helper()
	stackRef := "stack"
	cf := configFile

	parent := &configEnvCmd{
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
			env := escEnv
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
		},
		stackRef:   &stackRef,
		configFile: &cf,
	}
	parent.initArgs()

	return &configEnvRollbackCmd{parent: parent, runEnv: runEnv}
}

func TestConfigEnvRollbackDelegates(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	cmd := newRollbackCmdForTest(t, "testProject/testStack", "",
		func(_ context.Context, args []string) error {
			gotArgs = args
			return nil
		})

	require.NoError(t, cmd.run(t.Context(), "3"))
	require.Equal(t, []string{"version", "rollback", "org/testProject/testStack@3"}, gotArgs)
}

func TestConfigEnvRollbackRejectsPinned(t *testing.T) {
	t.Parallel()

	called := false
	cmd := newRollbackCmdForTest(t, "testProject/testStack@5", "",
		func(context.Context, []string) error {
			called = true
			return nil
		})

	err := cmd.run(t.Context(), "3")
	require.ErrorContains(t, err, "unpin")
	require.False(t, called, "a pinned stack must not dispatch a rollback")
}

func TestConfigEnvRollbackRejectsConfigFile(t *testing.T) {
	t.Parallel()

	called := false
	cmd := newRollbackCmdForTest(t, "testProject/testStack", "Pulumi.local.yaml",
		func(context.Context, []string) error {
			called = true
			return nil
		})

	err := cmd.run(t.Context(), "3")
	require.ErrorContains(t, err, "only supported for remote")
	require.False(t, called, "a local --config-file stack must not dispatch a rollback")
}
