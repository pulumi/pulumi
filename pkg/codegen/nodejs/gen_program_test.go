package nodejs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t, ProgramCodegenOptions)
}
