package python

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
)

var testdataPath = filepath.Join("..", "internal", "test", "testdata")

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t, "python", GenerateProgram)
}
