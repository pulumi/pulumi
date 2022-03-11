package gen

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestGenerateProgram(t *testing.T) {
	t.Parallel()

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check:      goCheck,
			GenProgram: GenerateProgram,
			TestCases:  test.PulumiPulumiProgramTests,
		})
}

func TestCollectImports(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))
	pulumiImports := codegen.NewStringSet()
	stdImports := codegen.NewStringSet()
	preambleHelperMethods := codegen.NewStringSet()
	g.collectImports(g.program, stdImports, pulumiImports, preambleHelperMethods)
	stdVals := stdImports.SortedValues()
	pulumiVals := pulumiImports.SortedValues()
	assert.Equal(t, 0, len(stdVals))
	assert.Equal(t, 1, len(pulumiVals))
	assert.Equal(t, "\"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3\"", pulumiVals[0])
}

func newTestGenerator(t *testing.T, testFile string) *generator {
	path := filepath.Join(testdataPath, testFile)
	contents, err := ioutil.ReadFile(path)
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

func goCheck(t *testing.T, path string, _ codegen.StringSet) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	// We remove go.mod to ensure tests are reproducible.
	goMod := filepath.Join(dir, "go.mod")
	if err = os.Remove(goMod); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	err = integration.RunCommand(t, "generate go.mod",
		[]string{ex, "mod", "init", "main"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	err = integration.RunCommand(t, "go tidy",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	err = integration.RunCommand(t, "point towards local Go SDK",
		[]string{ex, "mod", "edit",
			fmt.Sprintf("--replace=%s=%s",
				"github.com/pulumi/pulumi/sdk/v3",
				"../../../../../../../sdk")},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	err = integration.RunCommand(t, "go tidy after replace",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)

	err = integration.RunCommand(t, "test build", []string{ex, "build", "-v", "all"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	os.Remove(filepath.Join(dir, "main"))
	assert.NoError(t, err)
}
