package python

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestGenerateProgram(t *testing.T) {
	t.Parallel()

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      Check,
			GenProgram: GenerateProgram,
			TestCases:  test.PulumiPulumiProgramTests,
		})
}

func TestGenerateComponentResource(t *testing.T) {
	t.Parallel()

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__init__.py",
			Check:      Check,
			GenProgram: GenerateComponentResource,
			TestCases:  test.PulumiPulumiComponentResourceTests,
		})
}
