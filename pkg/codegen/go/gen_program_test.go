package gen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options-4.26": {
			Pkg:          "github.com/pulumi/pulumi-aws/sdk/v4",
			OpAndVersion: "v4.26.0",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "github.com/pulumi/pulumi-aws/sdk/v5",
			OpAndVersion: "v5.16.2",
		},
		"modpath": {
			Pkg:          "git.example.org/thirdparty/sdk",
			OpAndVersion: "v0.1.0",
		},
	}

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, "../../../../../../../sdk")
			},
			GenProgram: func(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
				// Prevent tests from interfering with each other
				return GenerateProgramWithOptions(program, GenerateProgramOptions{ExternalCache: NewCache()})
			},
			TestCases: []test.ProgramTest{
				{
					Directory:   "aws-resource-options-4.26",
					Description: "Resource Options",
				},
				{
					Directory:   "aws-resource-options-5.16.2",
					Description: "Resource Options",
				},
				{
					Directory:   "modpath",
					Description: "Check that modpath is respected",
					MockPluginVersions: map[string]string{
						"other": "0.1.0",
					},
					// We don't compile because the test relies on the `other` package,
					// which does not exist.
					SkipCompile: codegen.NewStringSet("go"),
				},
			},

			IsGenProject:    true,
			GenProject:      GenerateProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "go.mod",
		})
}

func TestCollectImports(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))

	programImports := g.collectImports(g.program)
	pulumiImports := programImports.pulumiImports
	stdImports := programImports.stdImports
	stdVals := stdImports.SortedValues()
	pulumiVals := pulumiImports.SortedValues()
	assert.Equal(t, 0, len(stdVals))
	assert.Equal(t, 1, len(pulumiVals))
	assert.Equal(t, "\"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3\"", pulumiVals[0])
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
		scopeTraversalRoots: codegen.NewStringSet(),
		arrayHelpers:        make(map[string]*promptToInputArrayHelper),
	}
	g.Formatter = format.NewFormatter(g)
	return g
}
