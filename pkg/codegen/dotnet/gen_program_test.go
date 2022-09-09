package dotnet

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func TestGenerateProgram(t *testing.T) {
	t.Parallel()

	dotnetVersion, err := exec.Command("dotnet", "--version").Output()
	contract.AssertNoErrorf(err, "Error checking dotnet version")

	if strings.HasPrefix(string(dotnetVersion), "3.") {
		t.Skip("Skipping dotnet codegen tests for dotnet 3.x")
	}

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, "../../../../../../../sdk/dotnet/Pulumi")
			},
			GenProgram: GenerateProgram,
			TestCases:  test.PulumiPulumiProgramTests,
		},
	)
}
