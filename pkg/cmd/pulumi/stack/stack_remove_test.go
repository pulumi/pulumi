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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func newRemoveTestStack(managedEnv string) (*backend.MockStack, *backend.MockBackend) {
	be := &backend.MockBackend{URLF: func() string { return "https://api.pulumi.com" }}
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "org/proj/dev", NameV: tokens.MustParseStackName("dev"), ProjectV: "proj",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{ManagedEnvironment: managedEnv}
		},
		OrgNameF: func() string { return "org" },
		BackendF: func() backend.Backend { return be },
	}, be
}

func TestRemoveStackPrompt(t *testing.T) {
	t.Parallel()

	unmanaged, _ := newRemoveTestStack("")
	managed, _ := newRemoveTestStack("proj/dev")
	assert.Equal(t, "This will permanently remove the 'org/proj/dev' stack!", removeStackPrompt(unmanaged))
	assert.Equal(t, "This will permanently remove the 'org/proj/dev' stack and its ESC environment 'org/proj/dev'!",
		removeStackPrompt(managed))
}

func TestManagedEnvironmentRemovalWarning(t *testing.T) {
	t.Parallel()

	unmanaged, _ := newRemoveTestStack("")
	managed, _ := newRemoveTestStack("proj/dev")
	assert.Empty(t, ManagedEnvironmentRemovalWarning(unmanaged, false))
	assert.Empty(t, ManagedEnvironmentRemovalWarning(unmanaged, true))
	assert.Equal(t,
		"--remove removes this stack after the destroy. Pulumi Cloud deletes the stack's ESC environment 'org/proj/dev'.",
		ManagedEnvironmentRemovalWarning(managed, false))
	assert.Equal(t,
		"--remove removes this stack, and Pulumi Cloud deletes its ESC environment 'org/proj/dev', "+
			"if the destroy leaves no resources.",
		ManagedEnvironmentRemovalWarning(managed, true))
}

//nolint:paralleltest // Replaces cmdBackend.DefaultLoginManager and pkgWorkspace.Instance.
func TestStackRemoveResultNamesManagedEnvironment(t *testing.T) {
	const removed = "Stack 'org/proj/dev' has been removed!"
	const deletes = " Pulumi Cloud deletes the stack's ESC environment 'org/proj/dev'."
	tests := []struct {
		name       string
		managedEnv string
		args       []string
		inProject  bool
		stackFile  string
		want       string
	}{
		{name: "unmanaged", want: removed + "\n"},
		{name: "managed", managedEnv: "proj/dev", want: removed + deletes + "\n"},
		{
			name:       "managed with --preserve-config",
			managedEnv: "proj/dev",
			args:       []string{"--preserve-config"},
			inProject:  true,
			stackFile:  "environment:\n  values:\n    pulumiConfig: {}\n",
			want:       removed + deletes + " Pulumi.dev.yaml still holds the environment's definition.\n",
		},
		{
			name:       "managed with --preserve-config and no definition in the stack file",
			managedEnv: "proj/dev",
			args:       []string{"--preserve-config"},
			inProject:  true,
			stackFile:  "secretsprovider: passphrase\n",
			want:       removed + deletes + "\n",
		},
		{
			name:       "managed with --preserve-config outside the project",
			managedEnv: "proj/dev",
			args:       []string{"--preserve-config"},
			want:       removed + deletes + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if tt.inProject {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: go\n"), 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.dev.yaml"), []byte(tt.stackFile), 0o600))
			}
			stack, be := newRemoveTestStack(tt.managedEnv)
			be.ParseStackReferenceF = func(string) (backend.StackReference, error) { return stack.Ref(), nil }
			be.GetStackF = func(context.Context, backend.StackReference) (backend.Stack, error) { return stack, nil }
			be.RemoveStackF = func(context.Context, backend.Stack, bool, bool) (bool, error) { return false, nil }

			prevLM := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = mockLoginManager(be)
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = prevLM })
			prevWS := pkgWorkspace.Instance
			pkgWorkspace.Instance = &pkgWorkspace.MockContext{}
			t.Cleanup(func() { pkgWorkspace.Instance = prevWS })

			cmd := newStackRemoveCmd()
			cmd.SetArgs(append([]string{"org/proj/dev", "--yes"}, tt.args...))
			var out bytes.Buffer
			cmd.SetOut(&out)
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			assert.Equal(t, tt.want, strings.ReplaceAll(colors.Never.Colorize(out.String()), dir+string(filepath.Separator), ""))
		})
	}
}
