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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
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

// managedTestStack returns stack app/dev managing environment proj/renamed, so an encrypt request naming the stack's
// project or name fails the test.
func managedTestStack(t *testing.T, loc backend.StackConfigLocation) (*backend.MockStack, *[]string) {
	var plaintexts []string
	be := &backend.MockManagedEnvironmentsBackend{
		EncryptEnvironmentSecretsF: func(
			_ context.Context, org string, project string, name string, batch [][]byte,
		) ([]string, error) {
			assert.Equal(t, "acme/proj/renamed", org+"/"+project+"/"+name)
			require.Len(t, batch, 1)
			plaintexts = append(plaintexts, string(batch[0]))
			return []string{fmt.Sprintf("ct%d", len(plaintexts))}, nil
		},
	}
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
		BackendF: func() backend.Backend { return be },
	}, &plaintexts
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
			expected: `# top comment
environment:
  values:
    pulumiConfig:
      app:keep: kept # keep comment
      foo: short
      app:token:
        fn::secret:
          ciphertext: ct1
`,
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
			s, _ := managedTestStack(t, loc)

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

const (
	importingDefinition = "environment:\n  imports:\n    - proj/base\n  values:\n    pulumiConfig:"
	importingStackFile  = importingDefinition + "\n      app:local: x # local comment\n      foo: short\n"
)

func importedConfigEnv(names ...string) *esc.Environment {
	values := map[string]esc.Value{}
	for _, name := range names {
		values[name] = esc.Value{Value: "imported", Trace: esc.Trace{Def: esc.Range{Environment: "proj/base"}}}
	}
	return &esc.Environment{Properties: map[string]esc.Value{"pulumiConfig": esc.NewValue(values)}}
}

func listImportEnv() *esc.Environment {
	trace := esc.Trace{Def: esc.Range{Environment: "proj/base"}}
	list := esc.Value{Value: []esc.Value{esc.NewValue("a"), esc.NewValue("b")}, Trace: trace}
	return &esc.Environment{Properties: map[string]esc.Value{"pulumiConfig": esc.NewValue(map[string]esc.Value{
		"bar": {Value: list.Value, Trace: trace},
		"obj": {Value: map[string]esc.Value{"items": list}, Trace: trace},
	})}}
}

func TestManagedConfigImportedKeys(t *testing.T) {
	t.Parallel()
	gone := apitype.EnvironmentDiagnostics{
		{Summary: "unknown environment proj/gone"},
		{Summary: "unknown environment proj/old"},
	}
	cases := []struct {
		name     string
		file     string
		set      []string
		remove   []string
		path     bool
		override bool
		// checkFirst calls CheckSet before Set, as set --secret does before it encrypts.
		checkFirst bool
		imported   *esc.Environment
		diags      apitype.EnvironmentDiagnostics
		checkErr   error
		noCall     bool
		expected   string
		warning    string
		info       string
		err        string
	}{
		{
			name:       "CheckSet refuses an imported key",
			set:        []string{"app:bar", "v"},
			checkFirst: true,
			err:        "config key app:bar is defined by imported environment proj/base",
		},
		{
			name:       "CheckSet then Set prints the override note once",
			set:        []string{"app:bar", "v"},
			override:   true,
			checkFirst: true,
			expected:   importingStackFile + "      bar: v\n",
			info:       "app:bar overrides the value from imported environment proj/base",
		},
		{
			name: "set of an imported key errors",
			set:  []string{"app:bar", "v"},
			err: "config key app:bar is defined by imported environment proj/base; the CLI does not edit imported " +
				"environments. Set it in that environment with `pulumi env set proj/base pulumiConfig.bar <value>`, " +
				"remove the import, or pass `--override-import` to override it in this stack's environment",
		},
		{
			name:     "override writes an imported key under the import's spelling",
			set:      []string{"app:bar", "v"},
			override: true,
			expected: importingStackFile + "      bar: v\n",
			info:     "app:bar overrides the value from imported environment proj/base",
		},
		{
			name:     "override writes a path under an imported key",
			set:      []string{"app:bar.inner", "v"},
			path:     true,
			override: true,
			expected: importingStackFile + "      bar:\n        inner: v\n",
		},
		{
			name:     "override rejects a path into an imported list",
			set:      []string{"app:bar[0]", "x"},
			path:     true,
			override: true,
			imported: listImportEnv(),
			err: "config key app:bar reaches into a list that imported environment proj/base defines, and a local " +
				"list replaces the whole imported list; override the whole list instead of part of it",
		},
		{
			name:     "override rejects a path through an imported object into a list",
			set:      []string{"app:obj.items[1]", "x"},
			path:     true,
			override: true,
			imported: listImportEnv(),
			err:      "config key app:obj reaches into a list that imported environment proj/base defines",
		},
		{
			name:     "path into an imported list under a local object errors",
			file:     importingDefinition + "\n      obj:\n        added: x\n",
			set:      []string{"app:obj.items[0]", "z"},
			path:     true,
			imported: listImportEnv(),
			err:      "config key app:obj reaches into a list that imported environment proj/base defines",
		},
		{
			name:     "path rm of an imported list under a local object errors",
			file:     importingDefinition + "\n      obj:\n        added: x\n",
			remove:   []string{"app:obj.items"},
			path:     true,
			imported: listImportEnv(),
			err: "config key app:obj.items is defined by imported environment proj/base; the CLI does not edit " +
				"imported environments. Remove it from that environment with " +
				"`pulumi env rm proj/base pulumiConfig.obj.items`, or remove the import",
		},
		{
			name:     "override rejects a path into an imported list under a local object",
			file:     importingDefinition + "\n      obj:\n        added: x\n",
			set:      []string{"app:obj.items[0]", "z"},
			path:     true,
			override: true,
			imported: listImportEnv(),
			err:      "config key app:obj reaches into a list that imported environment proj/base defines",
		},
		{
			name:     "path into a local list that replaces an imported one is edited",
			file:     importingDefinition + "\n      obj:\n        items: [q]\n",
			set:      []string{"app:obj.items[0]", "z"},
			path:     true,
			imported: listImportEnv(),
			expected: importingDefinition + "\n      obj:\n        items: [z]\n",
		},
		{
			name:     "path into a local object that replaces an imported list is edited",
			file:     importingDefinition + "\n      obj:\n        items: {enabled: true}\n",
			set:      []string{"app:obj.items.enabled", "false"},
			path:     true,
			imported: listImportEnv(),
			expected: importingDefinition + "\n      obj:\n        items: {enabled: false}\n",
		},
		{
			name:     "override writes a new key under an imported object",
			set:      []string{"app:obj.added", "x"},
			path:     true,
			override: true,
			imported: listImportEnv(),
			expected: importingStackFile + "      obj:\n        added: x\n",
		},
		{
			name:     "override still fails on the other spelling",
			set:      []string{"app:foo", "long"},
			override: true,
			err:      "pulumiConfig sets foo locally and imported environment proj/base sets app:foo;",
		},
		{
			name:   "remove of an imported key errors",
			remove: []string{"app:bar"},
			err: "config key app:bar is defined by imported environment proj/base; the CLI does not edit imported " +
				"environments. Remove it from that environment with `pulumi env rm proj/base pulumiConfig.bar`, or " +
				"remove the import",
		},
		{
			name: "path set under an imported key errors",
			set:  []string{"app:bar.inner", "v"},
			path: true,
			err:  "Set it in that environment with `pulumi env set proj/base pulumiConfig.bar.inner <value>`",
		},
		{
			name:     "local override is edited",
			set:      []string{"app:local", "y"},
			expected: importingDefinition + "\n      app:local: y # local comment\n      foo: short\n",
		},
		{
			name: "other spelling in an import fails set",
			set:  []string{"app:foo", "long"},
			err:  "pulumiConfig sets foo locally and imported environment proj/base sets app:foo; use one spelling",
		},
		{
			name:     "removing a local override and the other spelling checks once",
			remove:   []string{"app:local", "app:foo"},
			expected: importingDefinition + " {}\n",
		},
		{
			name:     "check failure warns and writes",
			set:      []string{"app:new", "v"},
			checkErr: errors.New("403 forbidden"),
			expected: importingStackFile + "      app:new: v\n",
			warning:  "could not check imported environments for app:new: 403 forbidden; writing it to the local definition",
		},
		{
			name: "diagnostics in a check error warn on one line",
			set:  []string{"app:new", "v"},
			checkErr: &client.EnvironmentErrorResponse{Diagnostics: []client.EnvironmentDiagnostic{
				{
					Range:   &esc.Range{Environment: "proj/renamed", Begin: esc.Pos{Line: 2, Column: 7}},
					Summary: "environment 'acme/proj/gone' not found",
				},
			}},
			expected: importingStackFile + "      app:new: v\n",
			warning: "could not check imported environments for app:new: " +
				"proj/renamed:2:7: environment 'acme/proj/gone' not found; writing it to the local definition\n",
		},
		{
			name:     "diagnostics warn and write even an imported key",
			set:      []string{"app:bar", "v"},
			diags:    gone,
			expected: importingStackFile + "      app:bar: v\n",
			warning: "could not check imported environments for app:bar: unknown environment proj/gone; " +
				"unknown environment proj/old; writing it to the local definition",
		},
		{
			name:     "no imports makes no call",
			file:     "environment:\n  values:\n    pulumiConfig:\n      foo: short\n",
			set:      []string{"app:bar", "v"},
			noCall:   true,
			expected: "environment:\n  values:\n    pulumiConfig:\n      foo: short\n      app:bar: v\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.file == "" {
				c.file = importingStackFile
			}
			configFile := filepath.Join(t.TempDir(), "Pulumi.dev.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(c.file), 0o600))

			calls := 0
			s, _ := managedTestStack(t, backend.StackConfigLocation{ManagedEnvironment: "proj/renamed"})
			s.BackendF = func() backend.Backend {
				return &backend.MockEnvironmentsBackend{
					CheckYAMLEnvironmentF: func(
						_ context.Context, org string, yaml []byte,
					) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
						calls++
						require.Equal(t, "acme", org)
						require.Equal(t, "imports:\n    - proj/base\n", string(yaml))
						imported := c.imported
						if imported == nil {
							imported = importedConfigEnv("app:local", "app:foo", "bar")
						}
						return imported, c.diags, c.checkErr
					},
				}
			}

			ps, err := cmdStack.LoadProjectStack(t.Context(), cmdutil.Diag(), &workspace.Project{Name: "app"}, s, configFile)
			require.NoError(t, err)
			store, err := newManagedStore(s, "app", ps, configFile, c.override)
			require.NoError(t, err)
			var infos, warnings bytes.Buffer
			store.sink = diag.DefaultSink(&infos, &warnings, diag.FormatOptions{Color: colors.Never})

			if c.set != nil {
				key := config.MustParseKey(c.set[0])
				if c.checkFirst {
					err = store.CheckSet(t.Context(), key, c.path)
				}
				if err == nil {
					err = store.Set(t.Context(), key, config.NewValue(c.set[1]), c.path)
				}
			}
			for _, k := range c.remove {
				if err == nil {
					err = store.Remove(t.Context(), config.MustParseKey(k), c.path)
				}
			}
			if err == nil {
				err = store.Save(t.Context())
			}

			if c.noCall {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
			if c.warning == "" {
				require.Empty(t, warnings.String())
			} else {
				require.Contains(t, warnings.String(), c.warning)
			}
			if c.info != "" {
				require.Equal(t, 1, strings.Count(infos.String(), c.info), infos.String())
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

func TestNewConfigWriterRejectsConfigFile(t *testing.T) {
	t.Parallel()

	managed, _ := managedTestStack(t, backend.StackConfigLocation{ManagedEnvironment: "proj/dev"})

	_, err := NewConfigWriter(managed, &workspace.Project{Name: "proj"}, &workspace.ProjectStack{},
		"Pulumi.other.yaml", &failingEncrypterFactory{t: t})

	require.EqualError(t, err, "--config-file is not supported on stacks with an ESC-managed environment yet")
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
		{
			name: "set-all",
			command: func(ws pkgWorkspace.Context, stack *string, lm cmdBackend.LoginManager, file *string) *cobra.Command {
				return newConfigSetAllCmd(ws, stack, lm, &failingEncrypterFactory{t: t}, file)
			},
			args: []string{"--plaintext", "app:new=v"},
		},
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

func TestManagedConfigCommandsNotSupported(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		managed string
		command func(cmdBackend.LoginManager, pkgWorkspace.Context, *string, *string) *cobra.Command
		args    []string
		err     string
	}{
		{
			name:    "refresh",
			managed: "dev",
			command: func(lm cmdBackend.LoginManager, ws pkgWorkspace.Context, stack *string, file *string) *cobra.Command {
				return newConfigRefreshCmd(ws, stack, lm, file)
			},
			err: "`pulumi config refresh` is not supported on stacks with an ESC-managed environment yet",
		},
		{
			name:    "copy from a managed stack",
			managed: "dev",
			command: func(lm cmdBackend.LoginManager, ws pkgWorkspace.Context, stack *string, file *string) *cobra.Command {
				return newConfigCopyCmd(ws, stack, lm, file)
			},
			args: []string{"--dest", "other"},
			err:  "`pulumi config copy` from stack org/dev is not supported on stacks with an ESC-managed environment yet",
		},
		{
			name:    "copy to a managed stack",
			managed: "other",
			command: func(lm cmdBackend.LoginManager, ws pkgWorkspace.Context, stack *string, file *string) *cobra.Command {
				return newConfigCopyCmd(ws, stack, lm, file)
			},
			args: []string{"--dest", "other"},
			err:  "`pulumi config copy` to stack org/other is not supported on stacks with an ESC-managed environment yet",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			configFile := filepath.Join(t.TempDir(), "Pulumi.dev.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(managedStackFile), 0o600))
			lm, ws := managedStacksLoginManager(c.managed)
			stack := "dev"
			cmd := c.command(lm, ws, &stack, &configFile)
			cmd.SetArgs(c.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SilenceUsage = true

			err := cmd.ExecuteContext(t.Context())
			require.ErrorContains(t, err, c.err)
			written, readErr := os.ReadFile(configFile)
			require.NoError(t, readErr)
			require.Equal(t, managedStackFile, string(written))
		})
	}
}

func managedConfigEnvCmd(t *testing.T, stackYAML string, saved *string) *configEnvCmd {
	cmd := newConfigEnvCmdForTestWithCheckYAMLEnvironment(nil, io.Discard, "name: app\nruntime: yaml", stackYAML,
		func(context.Context, string, string, string, []byte) (apitype.EnvironmentDiagnostics, error) {
			t.Error("no environment may be created")
			return nil, nil
		},
		func(context.Context, string, []byte) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return &esc.Environment{}, nil, nil
		},
		saved)
	withManagedEnvironment(t, cmd)
	return cmd
}

func withManagedEnvironment(t *testing.T, cmd *configEnvCmd) {
	requireStack := cmd.requireStack
	cmd.requireStack = func(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
		name string, lopt cmdStack.LoadOption, opts display.Options, configFile string,
	) (backend.Stack, error) {
		s, err := requireStack(ctx, sink, ws, lm, name, lopt, opts, configFile)
		require.NoError(t, err)
		s.(*backend.MockStack).ConfigLocationF = func() backend.StackConfigLocation {
			return backend.StackConfigLocation{ManagedEnvironment: "proj/renamed"}
		}
		return s, nil
	}
}

func TestManagedConfigEnvCommands(t *testing.T) {
	t.Parallel()

	t.Run("init is not supported", func(t *testing.T) {
		t.Parallel()
		var saved string
		init := &configEnvInitCmd{
			parent:     managedConfigEnvCmd(t, managedStackFile, &saved),
			newCrypter: newBase64EvalCrypter,
			yes:        true,
		}
		err := init.run(t.Context(), nil)
		require.EqualError(t, err,
			"`pulumi config env init` is not supported on stacks with an ESC-managed environment yet")
		require.Empty(t, saved)
	})

	t.Run("ls lists only imports", func(t *testing.T) {
		t.Parallel()
		for stackYAML, expected := range map[string][]string{
			"environment:\n  values:\n    pulumiConfig: {}\n":                                               {},
			"environment:\n  imports:\n    - proj/base\n  values:\n    pulumiConfig:\n      app:foo: bar\n": {"proj/base"},
		} {
			var saved string
			var listed []string
			err := managedConfigEnvCmd(t, stackYAML, &saved).listStackEnvironments(t.Context(),
				func(_ io.Writer, imports []string) error {
					listed = imports
					return nil
				})
			require.NoError(t, err)
			require.ElementsMatch(t, expected, listed)
		}
	})

	t.Run("add to the stack init placeholder", func(t *testing.T) {
		t.Parallel()
		var saved string
		add := &configEnvAddCmd{
			parent: managedConfigEnvCmd(t, "environment:\n  values:\n    pulumiConfig: {}\n", &saved),
			yes:    true,
		}
		require.NoError(t, add.run(t.Context(), []string{"proj/base"}))
		require.Equal(t, "environment:\n  values:\n    pulumiConfig: {}\n  imports:\n    - proj/base\n", saved)
	})

	t.Run("remove keeps the values", func(t *testing.T) {
		t.Parallel()
		var saved string
		rm := &configEnvRmCmd{
			parent: managedConfigEnvCmd(t, "environment:\n  imports:\n    - proj/base\n    - proj/other\n"+
				"  values:\n    pulumiConfig:\n      app:foo: bar # kept\n", &saved),
			yes: true,
		}
		require.NoError(t, rm.run(t.Context(), []string{"proj/base"}))
		require.Equal(t, "environment:\n  imports:\n    - proj/other\n"+
			"  values:\n    pulumiConfig:\n      app:foo: bar # kept\n", saved)
	})
}

//nolint:paralleltest // t.Chdir: set-all reads the default stack file
func TestManagedConfigSetAll(t *testing.T) {
	cases := []struct {
		name       string
		file       string
		imported   *esc.Environment
		override   bool
		plaintext  []string
		secret     []string
		json       string
		plaintexts []string
		added      string
		err        string
	}{
		{
			name:     "imported key is refused before its secret is encrypted",
			file:     importingStackFile,
			imported: importedConfigEnv("bar"),
			secret:   []string{"app:bar=one"},
			err: "config key app:bar is defined by imported environment proj/base; the CLI does not edit imported " +
				"environments. Set it in that environment with `pulumi env set proj/base pulumiConfig.bar <value>`, " +
				"remove the import, or pass `--override-import` to override it in this stack's environment",
		},
		{
			name:       "override-import writes an imported key under the import's spelling",
			file:       importingStackFile,
			imported:   importedConfigEnv("bar"),
			override:   true,
			secret:     []string{"app:bar=one"},
			plaintexts: []string{"one"},
			added:      "      bar:\n        fn::secret:\n          ciphertext: ct1\n",
		},
		{
			name:       "secret flags",
			secret:     []string{"app:a=one", "app:b=two"},
			plaintexts: []string{"one", "two"},
			added: "      app:a:\n        fn::secret:\n          ciphertext: ct1\n" +
				"      app:b:\n        fn::secret:\n          ciphertext: ct2\n",
		},
		{
			name:       "json secret",
			json:       `{"app:a": {"value": "one", "secret": true}}`,
			plaintexts: []string{"one"},
			added:      "      app:a:\n        fn::secret:\n          ciphertext: ct1\n",
		},
		{
			name: "json secret object",
			json: `{"app:obj": {"value": "{\"k\":\"v\"}", "objectValue": {"k": "v"}, "secret": true}}`,
			err: `could not set --json config for "app:obj": secret object values are not supported for ` +
				"stack-managed environments; set each leaf with --path --secret",
		},
		{
			name: "json secret object with other secrets",
			json: `{"app:a": {"value": "one", "secret": true}, ` +
				`"app:obj": {"value": "{\"k\":\"v\"}", "objectValue": {"k": "v"}, "secret": true}}`,
			err: `could not set --json config for "app:obj": secret object values are not supported for ` +
				"stack-managed environments; set each leaf with --path --secret",
		},
		{
			name:      "plaintext flag",
			plaintext: []string{"app:p=one"},
			added:     "      app:p: one\n",
		},
		{
			name:  "json plaintext scalar",
			json:  `{"app:s": {"value": "x"}}`,
			added: "      app:s: x\n",
		},
		{
			name:  "json plaintext object",
			json:  `{"app:o": {"value": "{\"k\":\"v\"}", "objectValue": {"k": "v"}}}`,
			added: "      app:o:\n        k: v\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: app\nruntime: yaml\n"), 0o600))
			if c.file == "" {
				c.file = managedStackFile
			}
			stackFile := filepath.Join(dir, "Pulumi.dev.yaml")
			require.NoError(t, os.WriteFile(stackFile, []byte(c.file), 0o600))
			configFile := ""
			s, plaintexts := managedTestStack(t, backend.StackConfigLocation{ManagedEnvironment: "proj/renamed"})
			if c.imported != nil {
				be := s.BackendF().(*backend.MockManagedEnvironmentsBackend)
				be.CheckYAMLEnvironmentF = func(
					context.Context, string, []byte,
				) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
					return c.imported, nil, nil
				}
			}
			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(string) (*workspace.Project, string, error) {
					return &workspace.Project{Name: "app"}, "", nil
				},
			}
			lm := &cmdBackend.MockLoginManager{
				LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool,
					colors.Colorization, ...cmdBackend.LoginOption,
				) (backend.Backend, error) {
					return &backend.MockBackend{
						GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) { return s, nil },
					}, nil
				},
			}
			stackName := "dev"
			cmd := newConfigSetAllCmd(ws, &stackName, lm, &failingEncrypterFactory{t: t}, &configFile)
			cmd.SetContext(t.Context())
			for _, arg := range c.plaintext {
				require.NoError(t, cmd.PersistentFlags().Set("plaintext", arg))
			}
			for _, arg := range c.secret {
				require.NoError(t, cmd.PersistentFlags().Set("secret", arg))
			}
			if c.json != "" {
				require.NoError(t, cmd.PersistentFlags().Set("json", c.json))
			}
			if c.override {
				require.NoError(t, cmd.PersistentFlags().Set("override-import", "true"))
			}

			err := cmd.RunE(cmd, nil)
			written, readErr := os.ReadFile(stackFile)
			require.NoError(t, readErr)
			if c.err != "" {
				require.EqualError(t, err, c.err)
				require.Equal(t, c.file, string(written))
				require.Empty(t, *plaintexts)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.plaintexts, *plaintexts)
			expected := strings.Replace(c.file, "foo: short\n", "foo: short\n"+c.added, 1)
			require.Equal(t, expected, string(written))
		})
	}
}

type failingEncrypterFactory struct {
	t *testing.T
}

func (f *failingEncrypterFactory) GetEncrypter(
	context.Context, backend.Stack, *workspace.ProjectStack,
) (config.Encrypter, cmdStack.SecretsManagerState, error) {
	f.t.Error("the stack secrets manager must not be loaded")
	return nil, cmdStack.SecretsManagerUnchanged, errors.New("unexpected call")
}
