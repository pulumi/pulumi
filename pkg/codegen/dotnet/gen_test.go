package dotnet

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "dotnet", GeneratePackage)
}
