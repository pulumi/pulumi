package nodejs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
)

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t, "nodejs", GenerateProgram)
}
