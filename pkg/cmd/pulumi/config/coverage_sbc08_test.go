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
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func projectReadingWS() *pkgWorkspace.MockContext {
	return &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Name: "testProject"}, "", nil
		},
	}
}

// configEnvParentReturning builds a configEnvCmd whose requireStack always resolves to stack, so the
// hidden config env subcommands can be driven without a real backend.
func configEnvParentReturning(ws pkgWorkspace.Context, stack backend.Stack) *configEnvCmd {
	ref, configFile := "testStack", ""
	return &configEnvCmd{
		stdout:     &bytes.Buffer{},
		ws:         ws,
		stackRef:   &ref,
		configFile: &configFile,
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return stack, nil
		},
	}
}

func TestConfigEnvRollbackRun(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("delegates to env version rollback qualified with the org", func(t *testing.T) {
		t.Parallel()
		env := "proj/env"
		var got []string
		cmd := &configEnvRollbackCmd{
			parent: configEnvParentReturning(projectReadingWS(), remoteStackWithEscEnv(&env, &backend.MockBackend{})),
			runEnv: func(_ context.Context, args []string) error { got = args; return nil },
		}
		require.NoError(t, cmd.run(ctx, "5"))
		require.Equal(t, []string{"version", "rollback", "org/proj/env@5"}, got)
	})

	t.Run("rejects a local stack", func(t *testing.T) {
		t.Parallel()
		local := &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
		}
		cmd := &configEnvRollbackCmd{
			parent: configEnvParentReturning(projectReadingWS(), local),
			runEnv: func(context.Context, []string) error { return nil },
		}
		require.ErrorContains(t, cmd.run(ctx, "5"), "only supported for remote config")
	})

	t.Run("rejects a pinned stack", func(t *testing.T) {
		t.Parallel()
		env := "proj/env@3"
		cmd := &configEnvRollbackCmd{
			parent: configEnvParentReturning(projectReadingWS(), remoteStackWithEscEnv(&env, &backend.MockBackend{})),
			runEnv: func(context.Context, []string) error { return nil },
		}
		require.ErrorContains(t, cmd.run(ctx, "5"), "pinned")
	})

	t.Run("rejects a stack with no linked environment", func(t *testing.T) {
		t.Parallel()
		cmd := &configEnvRollbackCmd{
			parent: configEnvParentReturning(projectReadingWS(), remoteStackWithEscEnv(nil, &backend.MockBackend{})),
			runEnv: func(context.Context, []string) error { return nil },
		}
		require.ErrorContains(t, cmd.run(ctx, "5"), "no linked environment")
	})
}

// TestConfigEnvRollbackPinsBackendURL covers the branch that pins PULUMI_BACKEND_URL to the stack's
// own backend so env version rollback targets the right cloud.
//
//nolint:paralleltest // mutates PULUMI_BACKEND_URL
func TestConfigEnvRollbackPinsBackendURL(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "https://ambient.example.com")
	env := "proj/env"
	be := &cloudURLBackend{url: "https://stack.example.com"}
	var sawURL string
	cmd := &configEnvRollbackCmd{
		parent: configEnvParentReturning(projectReadingWS(), remoteStackWithEscEnv(&env, be)),
		runEnv: func(context.Context, []string) error { sawURL = os.Getenv("PULUMI_BACKEND_URL"); return nil },
	}
	require.NoError(t, cmd.run(t.Context(), "5"))
	require.Equal(t, "https://stack.example.com", sawURL)
	require.Equal(t, "https://ambient.example.com", os.Getenv("PULUMI_BACKEND_URL"))
}

func TestConfigEnvPinLink(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	loaderReturning := func(ps *workspace.ProjectStack, err error) *configEnvCmd {
		return &configEnvCmd{
			loadProjectStack: func(
				context.Context, diag.Sink, *workspace.Project, backend.Stack, string,
			) (*workspace.ProjectStack, error) {
				return ps, err
			},
		}
	}

	t.Run("carries over secrets metadata and writes the new link", func(t *testing.T) {
		t.Parallel()
		var saved *workspace.ProjectStack
		stack := &backend.MockStack{
			SaveRemoteF: func(_ context.Context, ps *workspace.ProjectStack) error { saved = ps; return nil },
		}
		cmd := &configEnvPinCmd{
			parent: loaderReturning(&workspace.ProjectStack{SecretsProvider: "passphrase", EncryptionSalt: "salt"}, nil),
		}
		require.NoError(t, cmd.link(ctx, stack, &workspace.Project{Name: "p"}, "proj/env@5"))
		require.NotNil(t, saved)
		require.Equal(t, "passphrase", saved.SecretsProvider)
		require.Equal(t, "salt", saved.EncryptionSalt)
		require.Nil(t, saved.Config)
	})

	t.Run("aborts when the current configuration cannot be read", func(t *testing.T) {
		t.Parallel()
		cmd := &configEnvPinCmd{parent: loaderReturning(nil, errors.New("load failed"))}
		err := cmd.link(ctx, &backend.MockStack{}, &workspace.Project{}, "proj/env")
		require.ErrorContains(t, err, "reading current stack configuration")
	})

	t.Run("surfaces a save failure", func(t *testing.T) {
		t.Parallel()
		stack := &backend.MockStack{
			SaveRemoteF: func(context.Context, *workspace.ProjectStack) error { return errors.New("nope") },
		}
		cmd := &configEnvPinCmd{parent: loaderReturning(&workspace.ProjectStack{}, nil)}
		err := cmd.link(ctx, stack, &workspace.Project{}, "proj/env")
		require.ErrorContains(t, err, "updating remote configuration link")
	})
}
