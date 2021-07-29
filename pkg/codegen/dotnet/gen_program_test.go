package dotnet

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
)

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t, "dotnet", GenerateProgram)
}
