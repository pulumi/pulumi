package gen

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/utils"
	"github.com/stretchr/testify/assert"
)

var testdataPath = filepath.Join("..", "internal", "test", "testdata")

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t, "go", GenerateProgram)
}

func TestCollectImports(t *testing.T) {
	g := newTestGenerator(t, "aws-s3-logging.pp")
	pulumiImports := codegen.NewStringSet()
	stdImports := codegen.NewStringSet()
	g.collectImports(g.program, stdImports, pulumiImports)
	stdVals := stdImports.SortedValues()
	pulumiVals := pulumiImports.SortedValues()
	assert.Equal(t, 0, len(stdVals))
	assert.Equal(t, 1, len(pulumiVals))
	assert.Equal(t, "\"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3\"", pulumiVals[0])
}

func newTestGenerator(t *testing.T, testFile string) *generator {
	files, err := ioutil.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	for _, f := range files {
		if filepath.Base(f.Name()) != testFile {
			continue
		}

		path := filepath.Join(testdataPath, f.Name())
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			t.Fatalf("could not read %v: %v", path, err)
		}

		parser := syntax.NewParser()
		err = parser.ParseFile(bytes.NewReader(contents), f.Name())
		if err != nil {
			t.Fatalf("could not read %v: %v", path, err)
		}
		if parser.Diagnostics.HasErrors() {
			t.Fatalf("failed to parse files: %v", parser.Diagnostics)
		}

		program, diags, err := hcl2.BindProgram(parser.Files, hcl2.PluginHost(utils.NewHost(testdataPath)))
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
	t.Fatalf("test file not found")
	return nil
}
