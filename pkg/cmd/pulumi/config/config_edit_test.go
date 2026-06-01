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
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestConfigEditDelegatesToEnvEdit(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cases := []struct {
		name        string
		escEnv      string
		editorFlag  string
		showSecrets bool
		wantArgs    []string
	}{
		{
			name:     "plain ref",
			escEnv:   "testProject/testStack",
			wantArgs: []string{"edit", "org/testProject/testStack"},
		},
		{
			name:        "passes editor and show-secrets through",
			escEnv:      "testProject/testStack",
			editorFlag:  "code --wait",
			showSecrets: true,
			wantArgs:    []string{"edit", "org/testProject/testStack", "--editor", "code --wait", "--show-secrets"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var gotArgs []string
			cmd := &configEditCmd{
				configFile:  new(string),
				editorFlag:  c.editorFlag,
				showSecrets: c.showSecrets,
				runEnvEdit: func(_ context.Context, args []string) error {
					gotArgs = args
					return nil
				},
			}
			require.NoError(t, cmd.editRemote(ctx, remoteEditStack(c.escEnv)))
			require.Equal(t, c.wantArgs, gotArgs)
		})
	}
}

// cloudURLBackend is a MockBackend that also reports a CloudURL, so editRemote pins PULUMI_BACKEND_URL.
type cloudURLBackend struct {
	backend.MockBackend
	url string
}

func (b *cloudURLBackend) CloudURL() string { return b.url }

//nolint:paralleltest // mutates PULUMI_BACKEND_URL
func TestConfigEditPinsBackendURL(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "https://ambient.example.com")

	env := "testProject/testStack"
	stack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return &cloudURLBackend{url: "https://stack.example.com"} },
	}

	var sawURL string
	cmd := &configEditCmd{
		configFile: new(string),
		runEnvEdit: func(context.Context, []string) error {
			sawURL = os.Getenv("PULUMI_BACKEND_URL")
			return nil
		},
	}
	require.NoError(t, cmd.editRemote(t.Context(), stack))
	require.Equal(t, "https://stack.example.com", sawURL, "edit must run against the stack's backend")
	require.Equal(t, "https://ambient.example.com", os.Getenv("PULUMI_BACKEND_URL"), "prior value restored")
}

func TestRunEnvEditDispatchResolves(t *testing.T) {
	t.Parallel()
	// Regression: cobra executes from the root, so the env subcommand path must be on the root's args.
	// Passing too many args makes `env edit` fail its own MaximumNArgs(1) validation, which proves
	// dispatch resolved to `env edit` (without contacting a backend) rather than rejecting "config".
	root := newEnvRoot([]string{"edit", "x", "y", "z"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(t.Context())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "unknown command")
}

func TestConfigEditNoLinkedEnv(t *testing.T) {
	t.Parallel()
	called := false
	cmd := &configEditCmd{
		configFile: new(string),
		runEnvEdit: func(context.Context, []string) error { called = true; return nil },
	}
	stack := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: nil}
		},
		OrgNameF: func() string { return "org" },
	}
	err := cmd.editRemote(t.Context(), stack)
	require.ErrorContains(t, err, "no linked environment")
	require.False(t, called)
}

func TestConfigEditRejectsPinned(t *testing.T) {
	t.Parallel()
	called := false
	cmd := &configEditCmd{
		configFile: new(string),
		runEnvEdit: func(context.Context, []string) error { called = true; return nil },
	}
	err := cmd.editRemote(t.Context(), remoteEditStack("testProject/testStack@7"))
	require.ErrorContains(t, err, "pinned")
	require.False(t, called, "edit must not be dispatched for a pinned stack")
}

// remoteEditStack builds a remote (ESC-backed) MockStack linked to escEnv in org "org".
func remoteEditStack(escEnv string) *backend.MockStack {
	env := escEnv
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("testStack")}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true, EscEnv: &env}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return &backend.MockBackend{} },
	}
}
