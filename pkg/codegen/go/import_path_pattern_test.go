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

package gen

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

// The "importpath" synthetic schema (tests/testdata/codegen/importpath-1.0.0.json) sets the Go
// language option importPathPattern = "{baseImportPath}/{module}/v2". Program generation must honor
// that pattern when emitting imports, e.g. azure-native's per-module versioned SDK layout. This is
// the coverage previously provided by the azure-native-v2-eventgrid program test, re-expressed
// against a synthetic provider so the corpus no longer carries the 11M+ azure-native schema.
const importPathPatternProgram = `
resource "example" "importpath:eventgrid:Resource" {
    name  = "example"
    value = "example"
}
`

// The import path generation substitutes {module} ("eventgrid") and {baseImportPath} into the
// pattern from the schema.
const importPathPatternExpectedImport = "example.com/pulumi-importpath-sdk/eventgrid/v2"

// A minimal stand-in for the generated provider SDK, exposing exactly the surface the generated
// program references. It lives at the module path the importPathPattern resolves to, so a go.mod
// replace can point the generated program's import at it. SDK generation does not reproduce the
// per-module versioned layout this pattern implies, so the stand-in is hand-written.
const importPathPatternSDKGoMod = `module ` + importPathPatternExpectedImport + `

go 1.25

require github.com/pulumi/pulumi/sdk/v3 v3.0.0
`

const importPathPatternSDKResource = `package eventgrid

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Resource struct {
	pulumi.CustomResourceState
}

type resourceArgs struct {
	Name  string ` + "`pulumi:\"name\"`" + `
	Value string ` + "`pulumi:\"value\"`" + `
}

type ResourceArgs struct {
	Name  pulumi.StringInput
	Value pulumi.StringInput
}

func (ResourceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*resourceArgs)(nil)).Elem()
}

func NewResource(
	ctx *pulumi.Context, name string, args *ResourceArgs, opts ...pulumi.ResourceOption,
) (*Resource, error) {
	var resource Resource
	err := ctx.RegisterResource("importpath:eventgrid:Resource", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}
`

// TestImportPathPattern is a self-contained Go-only conformance check for the importPathPattern
// language option. It generates a program against the synthetic schema, asserts the generated import
// path, and then compiles the program with the Go toolchain against a stand-in SDK mounted at the
// expected module path via go.mod replace directives.
func TestImportPathPattern(t *testing.T) {
	t.Parallel()

	program := bindImportPathProgram(t)

	files, gDiags, err := GenerateProgram(program)
	require.NoError(t, err)
	require.False(t, gDiags.HasErrors(), "generate diagnostics: %v", gDiags)
	main, ok := files["main.go"]
	require.True(t, ok, "expected generated main.go")

	// Assert the import path with the Go parser rather than a substring match.
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "main.go", main, parser.ImportsOnly)
	require.NoError(t, err)
	imports := make([]string, 0, len(parsed.Imports))
	for _, imp := range parsed.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	assert.Contains(t, imports, importPathPatternExpectedImport,
		"importPathPattern should produce a {module}/v2 import path")

	// Compile the generated program with the Go toolchain against the stand-in SDK.
	requireProgramCompiles(t, main)
}

// bindImportPathProgram binds the test program against a host serving only the synthetic
// "importpath" provider, so the provider stays scoped to this test rather than the shared NewHost.
func bindImportPathProgram(t *testing.T) *pcl.Program {
	t.Helper()
	pclParser := syntax.NewParser()
	require.NoError(t, pclParser.ParseFile(strings.NewReader(importPathPatternProgram), "importpath.pp"))
	require.False(t, pclParser.Diagnostics.HasErrors(), "parse diagnostics: %v", pclParser.Diagnostics)

	host := utils.NewContextWithProviders(testdataPath, utils.NewSchemaProvider("importpath", "1.0.0"))
	program, diags, err := pcl.BindProgram(pclParser.Files, schema.NewPluginLoader(host))
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "bind diagnostics: %v", diags)
	return program
}

func requireProgramCompiles(t *testing.T, main []byte) {
	t.Helper()

	pulumiSDK, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk"))
	require.NoError(t, err)

	dir := t.TempDir()
	sdkDir := filepath.Join(dir, "sdk")
	require.NoError(t, os.MkdirAll(sdkDir, 0o700))

	goMod := "module importpathtest\n\n" +
		"go 1.25\n\n" +
		"replace " + importPathPatternExpectedImport + " => ./sdk\n" +
		"replace github.com/pulumi/pulumi/sdk/v3 => " + pulumiSDK + "\n"

	writeFile(t, filepath.Join(dir, "go.mod"), goMod)
	writeFile(t, filepath.Join(dir, "main.go"), string(main))
	writeFile(t, filepath.Join(sdkDir, "go.mod"), importPathPatternSDKGoMod)
	writeFile(t, filepath.Join(sdkDir, "resource.go"), importPathPatternSDKResource)

	// go mod tidy resolves the pulumi SDK's transitive requirements from the module cache; go build
	// then proves the generated import path resolves and the program type-checks.
	runGo(t, dir, "mod", "tidy")
	runGo(t, dir, "build", "./...")
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "go %s failed:\n%s", strings.Join(args, " "), out)
}
