package dotnet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func Check(t *testing.T, path string, dependencies codegen.StringSet, pulumiSDKPath string) {
	var err error
	dir := filepath.Dir(path)

	ex, err := executable.FindExecutable("dotnet")
	require.NoError(t, err, "Failed to find dotnet executable")

	// We create a new cs-project each time the test is run.
	projectFile := filepath.Join(dir, filepath.Base(dir)+".csproj")
	programFile := filepath.Join(dir, "Program.cs")
	if err = os.Remove(projectFile); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	if err = os.Remove(programFile); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	err = integration.RunCommand(t, "create dotnet project",
		[]string{ex, "new", "console", "-f", "net6.0"}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to create C# project")

	// Remove Program.cs again generated from "dotnet new console"
	// because the generated C# program already has an entry point
	if err = os.Remove(programFile); !os.IsNotExist(err) {
		require.NoError(t, err)
	}

	// Add dependencies
	pkgs := dotnetDependencies(dependencies)
	if len(pkgs) != 0 {
		for _, pkg := range pkgs {
			pkg.install(t, ex, dir)
		}
		dep{"Pulumi", test.PulumiDotnetSDKVersion}.install(t, ex, dir)
	} else {
		// We would like this regardless of other dependencies, but dotnet
		// packages do not play well with package references.
		if pulumiSDKPath != "" {
			err = integration.RunCommand(t, "add sdk ref",
				[]string{ex, "add", "reference", pulumiSDKPath},
				dir, &integration.ProgramTestOptions{})
			require.NoError(t, err, "Failed to dotnet sdk package reference")
		} else {
			dep{"Pulumi", test.PulumiDotnetSDKVersion}.install(t, ex, dir)
		}
	}

	// Clean up build result
	defer func() {
		err = os.RemoveAll(filepath.Join(dir, "bin"))
		assert.NoError(t, err, "Failed to remove bin result")
		err = os.RemoveAll(filepath.Join(dir, "obj"))
		assert.NoError(t, err, "Failed to remove obj result")
	}()
	TypeCheck(t, path, dependencies, pulumiSDKPath)
}

func TypeCheck(t *testing.T, path string, dependencies codegen.StringSet, pulumiSDKPath string) {
	var err error
	dir := filepath.Dir(path)

	ex, err := executable.FindExecutable("dotnet")
	require.NoError(t, err)

	err = integration.RunCommand(t, "dotnet build",
		[]string{ex, "build", "--nologo"}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to build dotnet project")
}

type dep struct {
	Name    string
	Version string
}

func (pkg dep) install(t *testing.T, ex, dir string) {
	args := []string{ex, "add", "package", pkg.Name}
	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}
	err := integration.RunCommand(t, "Add package",
		args, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to add dependency %q %q", pkg.Name, pkg.Version)
}

// Converts from the hcl2 dependency format to the dotnet format.
//
// Example:
//
//	"aws" => {"Pulumi.Aws", 4.21.1}
//	"azure" => {"Pulumi.Azure", 4.21.1}
func dotnetDependencies(deps codegen.StringSet) []dep {
	result := make([]dep, len(deps))
	for i, d := range deps.SortedValues() {
		switch d {
		case "aws":
			result[i] = dep{"Pulumi.Aws", test.AwsSchema}
		case "azure-native":
			result[i] = dep{"Pulumi.AzureNative", test.AzureNativeSchema}
		case "azure":
			// TODO: update constant in test.AzureSchema to v5.x
			// because it has output-versioned function invokes
			result[i] = dep{"Pulumi.Azure", "5.12.0"}
		case "kubernetes":
			result[i] = dep{"Pulumi.Kubernetes", test.KubernetesSchema}
		case "random":
			result[i] = dep{"Pulumi.Random", test.RandomSchema}
		default:
			result[i] = dep{"Pulumi." + Title(d), ""}
		}
	}
	return result
}

func GenerateProgramBatchTest(t *testing.T, testCases []test.ProgramTest) {
	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, "")
			},
			GenProgram: GenerateProgram,
			TestCases:  testCases,
		},
	)
}
