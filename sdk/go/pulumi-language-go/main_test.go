// Copyright 2016, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/blang/semver"
	gocodegen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
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
			JSON: &plugin.PulumiPluginJSON{
				Name:     "aws",
				Resource: true,
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
			JSON: &plugin.PulumiPluginJSON{
				Name:     "aws",
				Resource: true,
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
			JSON: &plugin.PulumiPluginJSON{
				Name:     "aws",
				Resource: true,
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
			JSON: &plugin.PulumiPluginJSON{
				Name:     "aws",
				Resource: true,
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
			JSON: &plugin.PulumiPluginJSON{
				Name:     "kubernetes",
				Resource: true,
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
	host := newLanguageHost("0.0.0.0:0", progDir, "", "")
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
			Project:                "deprecated", //nolint:staticcheck
			Pwd:                    progDir,      //nolint:staticcheck
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

func (e *mockEngine) RequirePulumiVersion(ctx context.Context, req *pulumirpc.RequirePulumiVersionRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.RequirePulumiVersionResponse, error) {
	return &pulumirpc.RequirePulumiVersionResponse{}, nil
}

func TestCompileProgram(t *testing.T) {
	t.Parallel()

	t.Run("no .go files", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		_, err := compileProgram(
			t.Context(), &mockEngine{}, tmp, "", false /* withDebugFlags */, stdout, stderr)
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
			t.Context(), engineClient, tmp, expectedOut, false /* withDebugFlags */, stdout, stderr)
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
			t.Context(), &mockEngine{}, tmp, "", false /* withDebugFlags */, stdout, stderr)
		require.ErrorContains(t, err, "unable to run `go build`: exit status 1")
		require.Contains(t, stderr.String(), "main.go:3:1: syntax error")
	})
}

// testSchemaLoader serves a single bound package over the loader RPC. It has no other packages,
// so external references cannot be resolved against it.
type testSchemaLoader struct {
	pkg *schema.Package
}

func (l *testSchemaLoader) load() (*schema.Package, error) {
	if l.pkg == nil {
		return nil, errors.New("no packages available")
	}
	return l.pkg, nil
}

func (l *testSchemaLoader) LoadPackage(string, *semver.Version) (*schema.Package, error) {
	return l.load()
}

func (l *testSchemaLoader) LoadPackageV2(
	context.Context, *schema.PackageDescriptor,
) (*schema.Package, error) {
	return l.load()
}

func (l *testSchemaLoader) LoadPackageReference(string, *semver.Version) (schema.PackageReference, error) {
	pkg, err := l.load()
	if err != nil {
		return nil, err
	}
	return pkg.Reference(), nil
}

func (l *testSchemaLoader) LoadPackageReferenceV2(
	context.Context, *schema.PackageDescriptor,
) (schema.PackageReference, error) {
	pkg, err := l.load()
	if err != nil {
		return nil, err
	}
	return pkg.Reference(), nil
}

func TestLinkImportInstructions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// namespace is the schema's namespace, which the default module path is built from.
		namespace string
		// goInfo is the "go" entry of the schema's language map.
		goInfo map[string]any
		// expectedImport is the import path Link is expected to print.
		expectedImport string
		// expectedModule is the module path Link is expected to add a replace directive for.
		expectedModule string
	}{
		{
			name:           "no go language info",
			namespace:      "example",
			expectedImport: "github.com/example/pulumi-file/sdk/go/file",
			expectedModule: "github.com/example/pulumi-file/sdk/go",
		},
		{
			name:           "no go language info and no namespace",
			expectedImport: "example.com/pulumi-file/sdk/go/file",
			expectedModule: "example.com/pulumi-file/sdk/go",
		},
		{
			name:           "import base path only",
			namespace:      "example",
			goInfo:         map[string]any{"importBasePath": "github.com/example/File/sdk/go/File"},
			expectedImport: "github.com/example/File/sdk/go/File",
			expectedModule: "github.com/example/File/sdk/go",
		},
		{
			name:           "module path only",
			namespace:      "example",
			goInfo:         map[string]any{"modulePath": "github.com/example/File/sdk/go"},
			expectedImport: "github.com/example/File/sdk/go/file",
			expectedModule: "github.com/example/File/sdk/go",
		},
		{
			name:      "module path and import base path",
			namespace: "example",
			goInfo: map[string]any{
				"modulePath":     "github.com/example/File/sdk/go",
				"importBasePath": "github.com/example/File/sdk/go/File",
			},
			expectedImport: "github.com/example/File/sdk/go/File",
			expectedModule: "github.com/example/File/sdk/go",
		},
		{
			// The module path wins over the import base path, so the generated SDK holds its
			// root package one directory below the module root. The printed import path must
			// address that directory, not the import base path.
			name:      "module path that disagrees with the import base path",
			namespace: "example",
			goInfo: map[string]any{
				"modulePath":     "github.com/example/File/sdk",
				"importBasePath": "github.com/example/File/sdk/go/File",
			},
			expectedImport: "github.com/example/File/sdk/File",
			expectedModule: "github.com/example/File/sdk",
		},
	}

	bind := func(t *testing.T, namespace string, goInfo map[string]any) *schema.Package {
		t.Helper()
		spec := schema.PackageSpec{
			Name:      "file",
			Namespace: namespace,
			Version:   "0.1.0",
			Meta:      &schema.MetadataSpec{SupportPack: true},
		}
		if goInfo != nil {
			raw, err := json.Marshal(goInfo)
			require.NoError(t, err)
			spec.Language = map[string]schema.RawMessage{"go": raw}
		}
		pkg, diags, err := schema.BindSpec(spec, &testSchemaLoader{}, schema.ValidationOptions{})
		require.NoError(t, err)
		require.False(t, diags.HasErrors(), "%v", diags)
		return pkg
	}

	// The replace directive uses a relative path, which must start with the separator of the
	// host platform. Build the same prefix that Link builds.
	relativeStart := "./"
	if runtime.GOOS == "windows" {
		relativeStart = ".\\"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cancel := make(chan bool)
			t.Cleanup(func() { close(cancel) })
			handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
				Init: func(srv *grpc.Server) error {
					loader := &testSchemaLoader{pkg: bind(t, tt.namespace, tt.goInfo)}
					schema.LoaderRegistration(schema.NewLoaderServer(loader))(srv)
					return nil
				},
				Cancel: cancel,
			})
			require.NoError(t, err)

			programDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(programDir, "go.mod"),
				[]byte("module example.com/program\n\ngo 1.25\n"), 0o600))

			host := &goLanguageHost{}
			resp, err := host.Link(t.Context(), &pulumirpc.LinkRequest{
				Info: &pulumirpc.ProgramInfo{
					RootDirectory:    programDir,
					ProgramDirectory: programDir,
				},
				LoaderTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
				Packages: []*pulumirpc.LinkRequest_LinkDependency{{
					Package: &pulumirpc.PackageDependency{Name: "file", Version: "0.1.0"},
					Path:    "sdks/example-file",
				}},
			})
			require.NoError(t, err)
			require.Contains(t, resp.ImportInstructions, "\""+tt.expectedImport+"\"")

			gomodContent, err := os.ReadFile(filepath.Join(programDir, "go.mod"))
			require.NoError(t, err)
			gomod, err := modfile.Parse("go.mod", gomodContent, nil)
			require.NoError(t, err)
			replaced := map[string]string{}
			for _, replace := range gomod.Replace {
				replaced[replace.Old.Path] = replace.New.Path
			}
			require.Equal(t, relativeStart+"sdks/example-file", replaced[tt.expectedModule])

			// The printed import path must address the root package of the SDK that the
			// generator writes, so that a program which follows the instructions compiles
			// against the linked SDK.
			files, err := gocodegen.GeneratePackage("pulumi-language-go", bind(t, tt.namespace, tt.goInfo), nil)
			require.NoError(t, err)
			generated, err := modfile.Parse("go.mod", files["go.mod"], nil)
			require.NoError(t, err)
			require.Equal(t, tt.expectedModule, generated.Module.Mod.Path)

			// The generator writes exactly one pulumi-plugin.json, in the directory of the
			// root package. Read that directory from it.
			var rootDirs []string
			for file := range files {
				if path.Base(file) == "pulumi-plugin.json" {
					rootDirs = append(rootDirs, path.Dir(file))
				}
			}
			require.Len(t, rootDirs, 1)
			require.Equal(t, tt.expectedImport, path.Join(generated.Module.Mod.Path, rootDirs[0]))
		})
	}
}
