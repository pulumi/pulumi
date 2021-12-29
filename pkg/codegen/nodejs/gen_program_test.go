package nodejs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check:      nodejsCheck,
			GenProgram: GenerateProgram,
		})
}

func nodejsCheck(t *testing.T, path string, dependencies codegen.StringSet) {
	ex, err := executable.FindExecutable("yarn")
	require.NoError(t, err, "Could not find yarn executable")
	dir := filepath.Dir(path)
	pkgs := nodejsPackages(dependencies)
	// We delete and regenerate package files for each run.
	packageJSON := filepath.Join(dir, "package.json")
	if err := os.Remove(packageJSON); !os.IsNotExist(err) {
		require.NoError(t, err, "Failed to delete %s", packageJSON)
	}
	yarnLock := filepath.Join(dir, "yarn.lock")
	if err := os.Remove(yarnLock); !os.IsNotExist(err) {
		require.NoError(t, err, "Failed to delete %s", yarnLock)
	}

	err = integration.RunCommand(t, "link @pulumi/pulumi",
		[]string{ex, "link", "@pulumi/pulumi"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to link @pulumi/pulumi")
	for _, pkg := range pkgs {
		err = integration.RunCommand(t, "yarn add and install",
			[]string{ex, "add", pkg}, dir, &integration.ProgramTestOptions{})
		require.NoError(t, err, "Could not install package: %q", pkg)
	}
	err = integration.RunCommand(t, "tsc check",
		[]string{ex, "run", "tsc", "--noEmit", filepath.Base(path)}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to build %q", path)
}

// Returns the nodejs equivalent to the hcl2 package names provided.
func nodejsPackages(deps codegen.StringSet) []string {
	if len(deps) == 0 {
		return []string{"@pulumi/pulumi@3.7.0"}
	}
	result := make([]string, len(deps))
	for i, d := range deps.SortedValues() {
		r := fmt.Sprintf("@pulumi/%s", d)
		v := func(s string) {
			r = fmt.Sprintf("%s@%s", r, s)
		}
		switch d {
		case "aws":
			v(test.AwsSchema)
		case "azure-native":
			v(test.AzureNativeSchema)
		case "azure":
			v(test.AzureSchema)
		case "kubernetes":
			v(test.KubernetesSchema)
		case "random":
			v(test.RandomSchema)
		}
		result[i] = r
	}
	return result
}
