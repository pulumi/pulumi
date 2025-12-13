// Copyright 2016-2025, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestParseRunParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		give    []string
		want    runParams
		wantErr string // non-empty if we expect an error
	}{
		{
			desc: "no arguments",
		},
		{
			desc: "no options",
			give: []string{"localhost:1234"},
			want: runParams{
				engineAddress: "localhost:1234",
			},
		},
		{
			desc: "tracing",
			give: []string{"-tracing", "foo.trace", "localhost:1234"},
			want: runParams{
				tracing:       "foo.trace",
				engineAddress: "localhost:1234",
			},
		},
		{
			desc:    "unknown option",
			give:    []string{"-unknown-option", "bar", "localhost:1234"},
			wantErr: "flag provided but not defined: -unknown-option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// Use a FlagSet with ContinueOnError for each case
			// instead of using the global flag set.
			//
			// The global flag set uses flag.ExitOnError,
			// so it cannot validate error cases during tests.
			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			fset.SetOutput(iotest.LogWriter(t))

			got, err := parseRunParams(fset, tt.give)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, &tt.want, got)
			}
		})
	}
}

func TestGetPackage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name          string
		Mod           *modInfo
		Expected      *pulumirpc.PackageDependency
		ExpectedError string
		JSON          *plugin.PulumiPluginJSON
		JSONPath      string
	}{
		{
			Name: "valid-pulumi-mod",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "v1.29.0",
			},
			Expected: &pulumirpc.PackageDependency{
				Name:    "aws",
				Version: "v1.29.0",
			},
		},
		{
			Name: "pulumi-pseduo-version-plugin",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "v1.29.1-0.20200403140640-efb5e2a48a86",
			},
			Expected: &pulumirpc.PackageDependency{
				Name:    "aws",
				Version: "v1.29.0",
			},
		},
		{
			Name: "non-pulumi-mod",
			Mod: &modInfo{
				Path:    "github.com/moolumi/pulumi-aws/sdk",
				Version: "v1.29.0",
			},
			ExpectedError: "module is not a pulumi provider",
		},
		{
			Name: "invalid-version-module",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "42-42-42",
			},
			ExpectedError: "module does not have semver compatible version",
		},
		{
			Name: "pulumi-pulumi-mod",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi/sdk",
				Version: "v1.14.0",
			},
			ExpectedError: "module is not a pulumi provider",
		},
		{
			Name: "beta-pulumi-module",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "v2.0.0-beta.1",
			},
			Expected: &pulumirpc.PackageDependency{
				Name:    "aws",
				Version: "v2.0.0-beta.1",
			},
		},
		{
			Name: "non-zero-patch-module", Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-kubernetes/sdk",
				Version: "v1.5.8",
			},
			Expected: &pulumirpc.PackageDependency{
				Name:    "kubernetes",
				Version: "v1.5.8",
			},
		},
		{
			Name: "pulumiplugin",
			Mod: &modInfo{
				Path:    "github.com/me/myself/i",
				Version: "invalid-Version",
			},
			Expected: &pulumirpc.PackageDependency{
				Name:    "thing1",
				Version: "v1.2.3",
				Server:  "myserver.com",
			},
			JSON: &plugin.PulumiPluginJSON{
				Resource: true,
				Name:     "thing1",
				Version:  "v1.2.3",
				Server:   "myserver.com",
			},
		},
		{
			Name:          "non-resource",
			Mod:           &modInfo{},
			ExpectedError: "module is not a pulumi provider",
			JSON: &plugin.PulumiPluginJSON{
				Resource: false,
			},
		},
		{
			Name: "missing-pulumiplugin",
			Mod: &modInfo{
				Dir: "/not/real",
			},
			ExpectedError: "module is not a pulumi provider",
			JSON: &plugin.PulumiPluginJSON{
				Name:    "thing2",
				Version: "v1.2.3",
			},
		},
		{
			Name: "pulumiplugin-go-lookup",
			Mod: &modInfo{
				Path:    "github.com/me/myself",
				Version: "v1.2.3",
			},
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			JSONPath: "go",
			Expected: &pulumirpc.PackageDependency{
				Name:    "name",
				Version: "v1.2.3",
			},
		},
		{
			Name: "pulumiplugin-go-name-lookup",
			Mod: &modInfo{
				Path:    "github.com/me/myself",
				Version: "v1.2.3",
			},
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			JSONPath: filepath.Join("go", "name"),
			Expected: &pulumirpc.PackageDependency{
				Name:    "name",
				Version: "v1.2.3",
			},
		},
		{
			Name: "pulumiplugin-nested-too-deep",
			Mod: &modInfo{
				Path:    "path.com/here",
				Version: "v0.0",
			},
			JSONPath: filepath.Join("go", "valid", "invalid"),
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			ExpectedError: "module is not a pulumi provider",
		},
		{
			Name: "nested-wrong-folder",
			Mod: &modInfo{
				Path:    "path.com/here",
				Version: "v0.0",
			},
			JSONPath: filepath.Join("invalid", "valid"),
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			ExpectedError: "module is not a pulumi provider",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			if c.Mod.Dir == "" {
				c.Mod.Dir = cwd
			}
			if c.JSON != nil {
				path := filepath.Join(cwd, c.JSONPath)
				err := os.MkdirAll(path, 0o700)
				require.NoErrorf(t, err, "Failed to setup test folder %s", path)
				bytes, err := c.JSON.JSON()
				require.NoError(t, err, "Failed to setup test pulumi-plugin.json")
				err = os.WriteFile(filepath.Join(path, "pulumi-plugin.json"), bytes, 0o600)
				require.NoError(t, err, "Failed to write pulumi-plugin.json")
			}

			actual, err := c.Mod.getPackage(t.TempDir())
			if c.ExpectedError != "" {
				assert.EqualError(t, err, c.ExpectedError)
			} else {
				// Kind must be resource. We can thus exclude it from the test.
				if c.Expected.Kind == "" {
					c.Expected.Kind = "resource"
				}
				require.NoError(t, err)
				assert.Equal(t, c.Expected, actual)
			}
		})
	}
}

func TestPluginsAndDependencies_moduleMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t,
		fsutil.CopyFile(root, filepath.Join("testdata", "sample"), nil),
		"copy test data")

	testPluginsAndDependencies(t, filepath.Join(root, "prog"))
}

// Test for https://github.com/pulumi/pulumi/issues/12526.
// Validates that if a Pulumi program has vendored its dependencies,
// the language host can still find the plugin and run the program.
func TestPluginsAndDependencies_vendored(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t,
		fsutil.CopyFile(root, filepath.Join("testdata", "sample"), nil),
		"copy test data")

	progDir := filepath.Join(root, "prog")

	// Vendor the dependencies and nuke the sources
	// to ensure that the language host can only use the vendored version.
	cmd := exec.Command("go", "mod", "vendor")
	cmd.Dir = progDir
	cmd.Stdout = iotest.LogWriter(t)
	cmd.Stderr = iotest.LogWriter(t)
	require.NoError(t, cmd.Run(), "vendor dependencies")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "plugin")))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "dep")))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "indirect-dep")))

	testPluginsAndDependencies(t, progDir)
}

// Regression test for https://github.com/pulumi/pulumi/issues/12963.
// Verifies that the language host can find plugins and dependencies
// when the Pulumi program is in a subdirectory of the project root.
func TestPluginsAndDependencies_subdir(t *testing.T) {
	t.Parallel()

	t.Run("moduleMode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		require.NoError(t,
			fsutil.CopyFile(root, filepath.Join("testdata", "sample"), nil),
			"copy test data")

		testPluginsAndDependencies(t, filepath.Join(root, "prog-subdir", "infra"))
	})

	t.Run("vendored", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		require.NoError(t,
			fsutil.CopyFile(root, filepath.Join("testdata", "sample"), nil),
			"copy test data")

		progDir := filepath.Join(root, "prog-subdir", "infra")

		// Vendor the dependencies and nuke the sources
		// to ensure that the language host can only use the vendored version.
		cmd := exec.Command("go", "mod", "vendor")
		cmd.Dir = progDir
		cmd.Stdout = iotest.LogWriter(t)
		cmd.Stderr = iotest.LogWriter(t)
		require.NoError(t, cmd.Run(), "vendor dependencies")
		require.NoError(t, os.RemoveAll(filepath.Join(root, "plugin")))
		require.NoError(t, os.RemoveAll(filepath.Join(root, "dep")))
		require.NoError(t, os.RemoveAll(filepath.Join(root, "indirect-dep")))

		testPluginsAndDependencies(t, progDir)
	})

	t.Run("gowork", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		require.NoError(t,
			fsutil.CopyFile(root, filepath.Join("testdata", "sample"), nil),
			"copy test data")

		testPluginsAndDependencies(t, filepath.Join(root, "prog-gowork", "prog"))
	})
}

func testPluginsAndDependencies(t *testing.T, progDir string) {
	host := newLanguageHost("0.0.0.0:0", progDir, "")
	ctx := t.Context()

	t.Run("GetRequiredPackages", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		res, err := host.GetRequiredPackages(ctx, &pulumirpc.GetRequiredPackagesRequest{
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    progDir,
				ProgramDirectory: progDir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)

		require.Len(t, res.Packages, 1)
		plug := res.Packages[0]

		assert.Equal(t, "example", plug.Name, "plugin name")
		assert.Equal(t, "v1.2.3", plug.Version, "plugin version")
		assert.Equal(t, "resource", plug.Kind, "plugin kind")
		assert.Equal(t, "example.com/download", plug.Server, "plugin server")
	})

	t.Run("GetProgramDependencies", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		res, err := host.GetProgramDependencies(ctx, &pulumirpc.GetProgramDependenciesRequest{
			Project:                "deprecated",
			Pwd:                    progDir,
			TransitiveDependencies: true,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    progDir,
				ProgramDirectory: progDir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)

		gotDeps := make(map[string]string) // name => version
		for _, dep := range res.Dependencies {
			gotDeps[dep.Name] = dep.Version
		}

		assert.Equal(t, map[string]string{
			"github.com/pulumi/go-dependency-testdata/plugin":          "v1.2.3",
			"github.com/pulumi/go-dependency-testdata/dep":             "v1.6.0",
			"github.com/pulumi/go-dependency-testdata/indirect-dep/v2": "v2.1.0",
		}, gotDeps)
	})
}

type mockEngine struct {
	logs []*pulumirpc.LogRequest
}

func (m *mockEngine) Log(ctx context.Context, in *pulumirpc.LogRequest,
	opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	m.logs = append(m.logs, in)
	return &emptypb.Empty{}, nil
}

func (m *mockEngine) GetRootResource(ctx context.Context, in *pulumirpc.GetRootResourceRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.GetRootResourceResponse, error) {
	return &pulumirpc.GetRootResourceResponse{}, nil
}

func (m *mockEngine) SetRootResource(ctx context.Context, in *pulumirpc.SetRootResourceRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.SetRootResourceResponse, error) {
	return &pulumirpc.SetRootResourceResponse{}, nil
}

func (m *mockEngine) StartDebugging(ctx context.Context, in *pulumirpc.StartDebuggingRequest,
	opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func TestCompileProgram(t *testing.T) {
	t.Parallel()

	t.Run("no .go files", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		_, err := compileProgram(
			context.Background(), &mockEngine{}, tmp, "", false /* withDebugFlags */, stdout, stderr)
		require.ErrorContains(t, err, "Failed to find go files")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		goMod := `module example`
		program := `package main
func main() {}
`
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		engineClient := &mockEngine{}
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goMod), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.go"), []byte(program), 0o600))
		expectedOut := filepath.Join(tmp, "out")
		out, err := compileProgram(
			context.Background(), engineClient, tmp, expectedOut, false /* withDebugFlags */, stdout, stderr)
		require.NoError(t, err)
		require.Equal(t, expectedOut, out)
		require.Len(t, engineClient.logs, 2)
		require.Equal(t, "Compiling the program ...", engineClient.logs[0].Message)
		require.Equal(t, "Finished compiling", engineClient.logs[1].Message)
	})

	t.Run("compile error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		goMod := `module example`
		badProgram := `package main
func main() {
`
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goMod), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.go"), []byte(badProgram), 0o600))
		_, err := compileProgram(
			context.Background(), &mockEngine{}, tmp, "", false /* withDebugFlags */, stdout, stderr)
		require.ErrorContains(t, err, "unable to run `go build`: exit status 1")
		require.Contains(t, stderr.String(), "main.go:3:1: syntax error")
	})
}
