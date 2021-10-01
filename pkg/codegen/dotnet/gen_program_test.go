package dotnet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func TestGenerateProgram(t *testing.T) {
	test.TestProgramCodegen(t,
		test.ProgramLangConfig{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "MyStack.cs",
			Check:      checkDotnet,
			GenProgram: GenerateProgram,
		},
	)
}

func checkDotnet(t *testing.T, path string) {
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
		[]string{ex, "new", "console"}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to create C# project")

	// Add dependencies (based on directory name)
	if pkg, pkgVersion := packagesFromTestName(dir); pkg != "" {
		err = integration.RunCommand(t, "Add package",
			[]string{ex, "add", "package", pkg, "--version", pkgVersion},
			dir, &integration.ProgramTestOptions{})
		require.NoError(t, err, "Failed to add dependency %q %q", pkg, pkgVersion)
	} else {
		err = integration.RunCommand(t, "add sdk ref",
			[]string{ex, "add", "reference", "../../../../../../../sdk/dotnet/Pulumi"},
			dir, &integration.ProgramTestOptions{})
		require.NoError(t, err, "Failed to dotnet sdk package reference")
	}

	// Clean up build result
	defer func() {
		err = os.RemoveAll(filepath.Join(dir, "bin"))
		assert.NoError(t, err, "Failed to remove bin result")
		err = os.RemoveAll(filepath.Join(dir, "obj"))
		assert.NoError(t, err, "Failed to remove obj result")
	}()
	err = integration.RunCommand(t, "dotnet build",
		[]string{ex, "build", "--nologo"}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to build dotnet project")
}

// packagesFromName attempts to figure out what package should be imported from
// the name of a test.
//
// Example:
// 	"aws-eks-pp" => ("Pulumi.Aws", 4.21.1)
// 	"azure-sa-pp" => ("Pulumi.Azure", 4.21.1)
// 	"resource-options-pp" => ("","")
//
// TODO[pulumi/pulumi#8080]
// Note: While we could instead do this by using the generateMetaData function
// for each language, we are trying not to expand the functionality under test.
func packagesFromTestName(name string) (string, string) {
	if strings.Contains(name, "aws") {
		return "Pulumi.Aws", test.AwsSchema
	} else if strings.Contains(name, "azure-native") {
		return "Pulumi.AzureNative", test.AzureNativeSchema
	} else if strings.Contains(name, "azure") {
		return "Pulumi.Azure", test.AzureSchema
	} else if strings.Contains(name, "kubernetes") {
		return "Pulumi.Kubernetes", test.KubernetesSchema
	} else if strings.Contains(name, "random") {
		return "Pulumi.Random", test.RandomSchema
	}
	return "", ""
}
