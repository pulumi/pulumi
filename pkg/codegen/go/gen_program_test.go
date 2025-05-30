// Copyright 2020-2024, Pulumi Corporation.
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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	rootDir, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)

	test.GenerateGoProgramTest(
		t,
		rootDir,
		func(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
			// Prevent tests from interfering with each other
			return GenerateProgramWithOptions(program, GenerateProgramOptions{ExternalCache: NewCache()})
		},
		GenerateProject,
	)
}

func TestCollectImports(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))
	g.collectImports(g.program)

	var allImports []string
	for _, group := range g.importer.ImportGroups() {
		allImports = append(allImports, group...)
	}

	assert.Equal(t, []string{
		`"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"`,
	}, allImports)
}

func TestFileImporter(t *testing.T) {
	t.Parallel()

	// importCall is a single call to fileImporter.Import.
	type importCall struct {
		// Import path to import.
		importPath string

		// Name of the package at the import path.
		name string

		// Expected name used to refer to the imported package.
		want string
	}

	tests := []struct {
		desc string

		// List of Import method invocations made in-order.
		imports []importCall

		// List of import groups generated from the collective effect
		// of all import calls in this test case.
		wantGroups [][]string
	}{
		{desc: "no imports"},
		{
			desc: "single import/std",
			imports: []importCall{
				{importPath: "fmt", name: "fmt", want: "fmt"},
			},
			wantGroups: [][]string{
				{`"fmt"`},
			},
		},
		{
			desc: "single import/pulumi",
			imports: []importCall{
				{importPath: "github.com/pulumi/pulumi/sdk/v3/go/pulumi", name: "pulumi", want: "pulumi"},
			},
			wantGroups: [][]string{
				{`"github.com/pulumi/pulumi/sdk/v3/go/pulumi"`},
			},
		},
		{
			desc: "std and pulumi/no conflict",
			imports: []importCall{
				{importPath: "fmt", name: "fmt", want: "fmt"},
				{importPath: "github.com/pulumi/pulumi/sdk/v3/go/pulumi", name: "pulumi", want: "pulumi"},
			},
			wantGroups: [][]string{
				{`"fmt"`},
				{`"github.com/pulumi/pulumi/sdk/v3/go/pulumi"`},
			},
		},
		{
			desc: "std and pulumi many imports, no conflict",
			imports: []importCall{
				{importPath: "fmt", name: "fmt", want: "fmt"},
				{importPath: "github.com/pulumi/pulumi/sdk/v3/go/pulumi", name: "pulumi", want: "pulumi"},
				{importPath: "github.com/pulumi/pulumi/sdk/v3/go/pulumi/config", name: "config", want: "config"},
				{importPath: "encoding/json", name: "json", want: "json"},
				{importPath: "github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3", name: "s3", want: "s3"},
				{importPath: "io", name: "io", want: "io"},
				{importPath: "github.com/pulumi/pulumi-awsx/sdk/v5/go/awsx", name: "awsx", want: "awsx"},
			},
			wantGroups: [][]string{
				{
					`"encoding/json"`,
					`"fmt"`,
					`"io"`,
				},
				{
					`"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"`,
					`"github.com/pulumi/pulumi-awsx/sdk/v5/go/awsx"`,
					`"github.com/pulumi/pulumi/sdk/v3/go/pulumi"`,
					`"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"`,
				},
			},
		},
		{
			desc: "std and pulumi/conflict",
			imports: []importCall{
				{importPath: "encoding/json", name: "json", want: "json"},
				{
					// This doesn't actually exist yet,
					// but it's conceivable that it might.
					importPath: "github.com/pulumi/pulumi-std/sdk/go/std/encoding/json",
					name:       "json",
					want:       "encodingjson",
				},
			},
			wantGroups: [][]string{
				{`"encoding/json"`},
				{`encodingjson "github.com/pulumi/pulumi-std/sdk/go/std/encoding/json"`},
			},
		},
		{
			desc: "std and pulumi/conflict repeated",
			imports: []importCall{
				{importPath: "encoding/json", name: "json", want: "json"},
				{
					importPath: "github.com/pulumi/pulumi-std/sdk/go/std/encoding/json",
					name:       "json",
					want:       "encodingjson",
				},
				{importPath: "encoding/json/v2", name: "json", want: "jsonv2"},
				{
					importPath: "github.com/pulumi/pulumi-std/sdk/v2/go/std/encoding/json",
					name:       "json",
					want:       "json2",
				},
			},
			wantGroups: [][]string{
				{
					`"encoding/json"`,
					`jsonv2 "encoding/json/v2"`,
				},
				{
					`encodingjson "github.com/pulumi/pulumi-std/sdk/go/std/encoding/json"`,
					`json2 "github.com/pulumi/pulumi-std/sdk/v2/go/std/encoding/json"`,
				},
			},
		},
		{
			desc: "std and pulumi/conflict reverse",
			imports: []importCall{
				{
					// This doesn't actually exist yet,
					// but it's conceivable that it might.
					importPath: "github.com/pulumi/pulumi-std/sdk/go/std/encoding/json",
					name:       "json",
					want:       "json",
				},
				{importPath: "encoding/json", name: "json", want: "json2"},
			},
			wantGroups: [][]string{
				{`json2 "encoding/json"`},
				{`"github.com/pulumi/pulumi-std/sdk/go/std/encoding/json"`},
			},
		},
		{
			desc: "pulumi aws awsx conflict",
			imports: []importCall{
				{importPath: "github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ecs", name: "ecs", want: "ecs"},
				{importPath: "github.com/pulumi/pulumi-awsx/sdk/go/awsx/ecs", name: "ecs", want: "awsxecs"},
			},
			wantGroups: [][]string{
				{
					`"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ecs"`,
					`awsxecs "github.com/pulumi/pulumi-awsx/sdk/go/awsx/ecs"`,
				},
			},
		},
		{
			desc: "basename mismatch/std",
			imports: []importCall{
				{importPath: "math/rand/v2", name: "rand", want: "rand"},
			},
			wantGroups: [][]string{
				{`rand "math/rand/v2"`},
			},
		},
		{
			desc: "basename mismatch/pulumi",
			imports: []importCall{
				{importPath: "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1", name: "corev1", want: "corev1"},
			},
			wantGroups: [][]string{
				{`corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"`},
			},
		},
		{
			desc: "basename mismatch/third party",
			imports: []importCall{
				{importPath: "gopkg.in/yaml.v3", name: "yaml", want: "yaml"},
			},
			wantGroups: [][]string{
				{`yaml "gopkg.in/yaml.v3"`},
			},
		},
		{
			desc: "already imported",
			imports: []importCall{
				{importPath: "example.com/foo/bar", name: "bar", want: "bar"},
				{importPath: "example.com/baz/bar", name: "bar", want: "bazbar"},

				// Reimport should get existing resolved names.
				{importPath: "example.com/foo/bar", name: "bar", want: "bar"},
				{importPath: "example.com/baz/bar", name: "bar", want: "bazbar"},
			},
			wantGroups: [][]string{
				{
					// Note:
					// Imports are sorted by import path,
					// not the order they were imported.
					`bazbar "example.com/baz/bar"`,
					`"example.com/foo/bar"`,
				},
			},
		},
		{
			desc: "many conflicts",
			imports: []importCall{
				{importPath: "example.com/foo/bar", name: "bar", want: "bar"},
				{importPath: "example.com/baz/bar", name: "bar", want: "bazbar"},
				{importPath: "example.com/qux/bar", name: "bar", want: "quxbar"},
				{importPath: "example.com/quux/bar", name: "bar", want: "quuxbar"},
			},
			wantGroups: [][]string{
				{
					// Note:
					// Imports are sorted by import path,
					// not the order they were imported.
					`bazbar "example.com/baz/bar"`,
					`"example.com/foo/bar"`,
					`quuxbar "example.com/quux/bar"`,
					`quxbar "example.com/qux/bar"`,
				},
			},
		},
		{
			desc: "conflict with special characters",
			imports: []importCall{
				{importPath: "example.com/foo/bar-go", name: "bar", want: "bar"},
				{importPath: "example.com/foo/bar.go", name: "bar", want: "foobargo"},
				{importPath: "example.com/bar", name: "bar", want: "bar2"}, // nothing to join with
				{importPath: "example.com/f-o-o/bar", name: "bar", want: "foobar"},
			},
			wantGroups: [][]string{
				{
					`bar2 "example.com/bar"`,
					`foobar "example.com/f-o-o/bar"`,
					`bar "example.com/foo/bar-go"`,
					`foobargo "example.com/foo/bar.go"`,
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fimp := newFileImporter()
			for _, imp := range tt.imports {
				gotName := fimp.Import(imp.importPath, imp.name)
				assert.Equal(t, imp.want, gotName, "Import(%q, %q)", imp.importPath, imp.name)
			}

			assert.Equal(t, tt.wantGroups, fimp.ImportGroups())
		})
	}
}

func TestFileImporter_Reset(t *testing.T) {
	t.Parallel()

	fimp := newFileImporter()
	assert.Empty(t, fimp.ImportGroups())

	// Add imports.
	assert.Equal(t, "bar", fimp.Import("example.com/foo/bar", "bar"))
	assert.Equal(t, "bazbar", fimp.Import("example.com/baz/bar", "bar"))
	assert.NotEmpty(t, fimp.ImportGroups()) // sanity check

	// Reset and check that imports are gone.
	fimp.Reset()
	assert.Empty(t, fimp.ImportGroups())

	// Import again in reverse order.
	// Prior state shouldn't affect result.
	assert.Equal(t, "bar", fimp.Import("example.com/baz/bar", "bar"),
		"should not get alias after reset")
	assert.Equal(t, "foobar", fimp.Import("example.com/foo/bar", "bar"))
}

func TestToIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want string
		ok   bool
	}{
		{"foo", "foo", true},
		{"foo-bar", "foobar", true},
		{"foo_bar", "foo_bar", true},
		{"foo.bar", "foobar", true},
		{"foo/123", "foo123", true},
		{"foo.123/bar", "foo123bar", true},
		{"123", "", false},
		{"1/foo", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			got, ok := toIdentifier(tt.give)
			require.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSecondLastIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		haystack string
		needle   string
		want     int
	}{
		{
			desc:     "empty",
			haystack: "",
			needle:   "foo",
			want:     -1,
		},
		{
			desc:     "no match",
			haystack: "foo",
			needle:   "bar",
			want:     -1,
		},
		{
			desc:     "one match",
			haystack: "a/b",
			needle:   "/",
			want:     -1,
		},
		{
			desc:     "two matches",
			haystack: "a/b/c",
			needle:   "/",
			want:     1,
		},
		{
			desc:     "three matches",
			haystack: "a/b/c/d",
			needle:   "/",
			want:     3,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := secondLastIndex(tt.haystack, tt.needle)
			assert.Equal(t, tt.want, got)
		})
	}
}

func newTestGenerator(t *testing.T, testFile string) *generator {
	path := filepath.Join(testdataPath, testFile)
	contents, err := os.ReadFile(path)
	require.NoErrorf(t, err, "could not read %v: %v", path, err)

	parser := syntax.NewParser()
	err = parser.ParseFile(bytes.NewReader(contents), filepath.Base(path))
	if err != nil {
		t.Fatalf("could not read %v: %v", path, err)
	}
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	program, diags, err := pcl.BindProgram(parser.Files, pcl.PluginHost(utils.NewHost(testdataPath)))
	if err != nil {
		t.Fatalf("could not bind program: %v", err)
	}
	if diags.HasErrors() {
		t.Fatalf("failed to bind program: %v", diags)
	}

	g := &generator{
		program:             program,
		jsonTempSpiller:     &jsonSpiller{},
		ternaryTempSpiller:  &tempSpiller{},
		readDirTempSpiller:  &readDirSpiller{},
		splatSpiller:        &splatSpiller{},
		optionalSpiller:     &optionalSpiller{},
		inlineInvokeSpiller: &inlineInvokeOrCallSpiller{},
		scopeTraversalRoots: codegen.NewStringSet(),
		arrayHelpers:        make(map[string]*promptToInputArrayHelper),
		importer:            newFileImporter(),
	}
	g.Formatter = format.NewFormatter(g)
	return g
}

func parseAndBindProgram(t *testing.T,
	text string,
	name string,
	options ...pcl.BindOption,
) (*pcl.Program, hcl.Diagnostics, error) {
	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(text), name)
	if err != nil {
		t.Fatalf("could not read %v: %v", name, err)
	}
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	options = append(options, pcl.PluginHost(utils.NewHost(testdataPath)))

	return pcl.BindProgram(parser.Files, options...)
}

func TestGenerateProjectDoesNotPanicWhenMissingVersion(t *testing.T) {
	t.Parallel()

	source := `
resource main "auto-deploy:index:AutoDeployer" {
    project = "example"
}`

	program, diags, err := parseAndBindProgram(t, source, "main.pp")

	require.NoError(t, err)
	require.False(t, diags.HasErrors())

	files, diags, err := GenerateProjectFiles(workspace.Project{}, program, nil)
	assert.NotNil(t, files, "Files were generated")
	require.NoError(t, err)
	require.False(t, diags.HasErrors())
}

func TestDeferredOutputTypeParameter(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "string", deferredOutputTypeParameter(model.StringType))
	assert.Equal(t, "bool", deferredOutputTypeParameter(model.BoolType))
	assert.Equal(t, "int", deferredOutputTypeParameter(model.IntType))
	assert.Equal(t, "float64", deferredOutputTypeParameter(model.NumberType))
	assert.Equal(t, "[]string", deferredOutputTypeParameter(model.NewListType(model.StringType)))
	assert.Equal(t, "[]int", deferredOutputTypeParameter(model.NewListType(model.IntType)))
	assert.Equal(t, "[]bool", deferredOutputTypeParameter(model.NewListType(model.BoolType)))
	assert.Equal(t, "[]float64", deferredOutputTypeParameter(model.NewListType(model.NumberType)))
	assert.Equal(t, "map[string]string", deferredOutputTypeParameter(model.NewMapType(model.StringType)))
	assert.Equal(t, "map[string]int", deferredOutputTypeParameter(model.NewMapType(model.IntType)))
	assert.Equal(t, "map[string]bool", deferredOutputTypeParameter(model.NewMapType(model.BoolType)))
	assert.Equal(t, "map[string]float64", deferredOutputTypeParameter(model.NewMapType(model.NumberType)))
	assert.Equal(t, "map[string][]string", deferredOutputTypeParameter(
		model.NewMapType(model.NewListType(model.StringType))))
	assert.Equal(t, "interface{}", deferredOutputTypeParameter(model.DynamicType))
}

func TestDeferredOutputCastTypeParameter(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "pulumi.AnyOutput", deferredOutputCastTypeParameter(model.DynamicType))
	assert.Equal(t, "pulumi.StringOutput", deferredOutputCastTypeParameter(model.StringType))
	assert.Equal(t, "pulumi.BoolOutput", deferredOutputCastTypeParameter(model.BoolType))
	assert.Equal(t, "pulumi.IntOutput", deferredOutputCastTypeParameter(model.IntType))
	assert.Equal(t, "pulumi.Float64Output", deferredOutputCastTypeParameter(model.NumberType))
	assert.Equal(t, "pulumi.StringArrayOutput", deferredOutputCastTypeParameter(model.NewListType(model.StringType)))
	assert.Equal(t, "pulumi.IntArrayOutput", deferredOutputCastTypeParameter(model.NewListType(model.IntType)))
	assert.Equal(t, "pulumi.BoolArrayOutput", deferredOutputCastTypeParameter(model.NewListType(model.BoolType)))
	assert.Equal(t, "pulumi.Float64ArrayOutput", deferredOutputCastTypeParameter(model.NewListType(model.NumberType)))
	assert.Equal(t, "pulumi.ArrayOutput", deferredOutputCastTypeParameter(model.NewListType(model.DynamicType)))
	assert.Equal(t, "pulumi.StringMapOutput", deferredOutputCastTypeParameter(model.NewMapType(model.StringType)))
	assert.Equal(t, "pulumi.IntMapOutput", deferredOutputCastTypeParameter(model.NewMapType(model.IntType)))
	assert.Equal(t, "pulumi.BoolMapOutput", deferredOutputCastTypeParameter(model.NewMapType(model.BoolType)))
	assert.Equal(t, "pulumi.Float64MapOutput", deferredOutputCastTypeParameter(model.NewMapType(model.NumberType)))
	assert.Equal(t, "pulumi.MapOutput", deferredOutputCastTypeParameter(model.NewMapType(model.DynamicType)))
}
