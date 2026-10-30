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
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const managedStackFile = `# top comment
environment:
  values:
    pulumiConfig:
      app:keep: kept # keep comment
      foo: short
`

func managedTestStack(t *testing.T, loc backend.StackConfigLocation) *backend.MockStack {
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
		OrgNameF:        func() string { return "acme" },
		ConfigLocationF: func() backend.StackConfigLocation { return loc },
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			t.Error("the stack secrets manager must not be loaded")
			return nil, nil
		},
	}
}

func TestManagedConfigWrites(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		loc      *backend.StackConfigLocation
		file     string
		set      []string
		remove   []string
		path     bool
		secret   bool
		expected string
		err      string
	}{
		{
			name: "new key keeps comments",
			set:  []string{"app:new", "true"},
			expected: `# top comment
environment:
  values:
    pulumiConfig:
      app:keep: kept # keep comment
      foo: short
      app:new: "true"
`,
		},
		{
			name: "existing short spelling is edited",
			set:  []string{"app:foo", "long"},
			expected: `# top comment
environment:
  values:
    pulumiConfig:
      app:keep: kept # keep comment
      foo: long
`,
		},
		{
			name: "path set",
			set:  []string{"app:obj.list[0]", "true"},
			path: true,
			expected: `# top comment
environment:
  values:
    pulumiConfig:
      app:keep: kept # keep comment
      foo: short
      app:obj:
        list:
          - true
`,
		},
		{
			name: "single-segment path set",
			set:  []string{"app:new", "v"},
			path: true,
			expected: `# top comment
environment:
  values:
    pulumiConfig:
      app:keep: kept # keep comment
      foo: short
      app:new: v
`,
		},
		{
			name: "path set starting with an index",
			set:  []string{"app:[0].a", "v"},
			path: true,
			err:  "first path segment of config key must be a non-empty string",
		},
		{
			name: "values under config",
			file: managedStackFile + "config:\n  app:old: legacy\n",
			set:  []string{"app:new", "v"},
			err:  "has values under `config:`",
		},
		{
			name:     "set into the stack init placeholder writes block style",
			file:     "environment:\n  values:\n    pulumiConfig: {}\n",
			set:      []string{"app:foo", "bar"},
			expected: "environment:\n  values:\n    pulumiConfig:\n      app:foo: bar\n",
		},
		{
			name:   "secret",
			set:    []string{"app:token", "hunter2"},
			secret: true,
			err:    "`pulumi config set --secret` is not supported on stacks with an ESC-managed environment yet",
		},
		{
			name: "empty definition",
			file: "environment: {}\n",
			set:  []string{"app:new", "v"},
			err:  "has no definition for managed environment acme/proj/renamed.",
		},
		{
			name:     "unmanaged inline environment writes config",
			loc:      &backend.StackConfigLocation{},
			file:     "environment:\n  values:\n    pulumiConfig:\n      foo: bar\n",
			set:      []string{"app:new", "v"},
			expected: "environment:\n  values:\n    pulumiConfig:\n      foo: bar\nconfig:\n  app:new: v\n",
		},
		{
			name:     "remove both spellings",
			remove:   []string{"app:keep", "app:foo"},
			expected: "# top comment\nenvironment:\n  values:\n    pulumiConfig: {}\n",
		},
		{
			name:     "path remove",
			file:     "environment:\n  values:\n    pulumiConfig:\n      app:obj:\n        a: 1\n        b: 2\n",
			remove:   []string{"app:obj.a"},
			path:     true,
			expected: "environment:\n  values:\n    pulumiConfig:\n      app:obj:\n        b: 2\n",
		},
		{
			name:     "single-segment path remove",
			remove:   []string{"app:keep"},
			path:     true,
			expected: "# top comment\nenvironment:\n  values:\n    pulumiConfig:\n      foo: short\n",
		},
		{
			name:   "path remove starting with an index",
			remove: []string{"app:[0]"},
			path:   true,
			err:    "first path segment of config key must be a non-empty string",
		},
		{
			name:   "remove with values under config",
			file:   managedStackFile + "config:\n  app:old: legacy\n",
			remove: []string{"app:keep"},
			err:    "has values under `config:`",
		},
		{
			name:     "unmanaged remove writes config",
			loc:      &backend.StackConfigLocation{},
			file:     "config:\n  app:a: x\n  app:b: y\n",
			remove:   []string{"app:a"},
			expected: "config:\n  app:b: y\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			loc := backend.StackConfigLocation{ManagedEnvironment: "proj/renamed"}
			if c.loc != nil {
				loc = *c.loc
			}
			if c.file == "" {
				c.file = managedStackFile
			}
			configFile := filepath.Join(t.TempDir(), "Pulumi.dev.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(c.file), 0o600))
			s := managedTestStack(t, loc)

			project := &workspace.Project{Name: "app"}
			var err error
			if c.remove != nil {
				ps, loadErr := cmdStack.LoadProjectStack(t.Context(), cmdutil.Diag(), project, s, configFile)
				require.NoError(t, loadErr)
				keys := make([]config.Key, 0, len(c.remove))
				for _, k := range c.remove {
					keys = append(keys, config.MustParseKey(k))
				}
				err = removeConfigKeys(t.Context(), s, project, ps, configFile, keys, c.path)
			} else {
				cmd := &configSetCmd{Path: c.path, Secret: c.secret, LoadProjectStack: cmdStack.LoadProjectStack}
				err = cmd.Run(t.Context(), &pkgWorkspace.MockContext{}, c.set, project, s, configFile)
			}
			written, readErr := os.ReadFile(configFile)
			require.NoError(t, readErr)
			if c.err != "" {
				require.ErrorContains(t, err, c.err)
				require.Equal(t, c.file, string(written))
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expected, string(written))
		})
	}
}

func managedStacksLoginManager(managed string) (cmdBackend.LoginManager, pkgWorkspace.Context) {
	be := &backend.MockBackend{
		ParseStackReferenceF: func(name string) (backend.StackReference, error) {
			return &backend.MockStackReference{StringV: "org/" + name, NameV: tokens.MustParseStackName(name)}, nil
		},
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			var loc backend.StackConfigLocation
			if ref.Name().String() == managed {
				loc.ManagedEnvironment = "proj/renamed"
			}
			return &backend.MockStack{
				RefF:            func() backend.StackReference { return ref },
				OrgNameF:        func() string { return "org" },
				ConfigLocationF: func() backend.StackConfigLocation { return loc },
			}, nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool,
			colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return be, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "app"}, "", nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{Current: "https://api.pulumi.com"}, nil
		},
	}
	return lm, ws
}

func TestManagedConfigWritesRejectConfigFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		command func(pkgWorkspace.Context, *string, cmdBackend.LoginManager, *string) *cobra.Command
		args    []string
	}{
		{name: "set", command: newConfigSetCmd, args: []string{"app:new", "v"}},
		{name: "rm", command: newConfigRemoveCmd, args: []string{"app:keep"}},
		{name: "rm-all", command: newConfigRemoveAllCmd, args: []string{"app:keep"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			configFile := filepath.Join(t.TempDir(), "Pulumi.alt.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(managedStackFile), 0o600))
			lm, ws := managedStacksLoginManager("dev")
			stack := "dev"
			cmd := c.command(ws, &stack, lm, &configFile)
			cmd.SetArgs(c.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SilenceUsage = true

			err := cmd.ExecuteContext(t.Context())
			require.EqualError(t, err, "--config-file is not supported on stacks with an ESC-managed environment yet")
			written, readErr := os.ReadFile(configFile)
			require.NoError(t, readErr)
			require.Equal(t, managedStackFile, string(written))
		})
	}
}
