package dotnet

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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
			Check: func(*testing.T, string) {
			},
			GenProgram: GenerateProgram,
		},
	)
}

func checkDotnet(t *testing.T, path string) {
	var err error
	dir := filepath.Dir(path)

	ex, err := executable.FindExecutable("dotnet")
	assert.NoError(t, err, "Failed to find dotnet executable")

	projectFile := filepath.Join(dir, filepath.Base(dir)+".csproj")
	if _, err := ioutil.ReadFile(projectFile); os.IsNotExist(err) {
		defer func() {
			err = os.Remove(projectFile)
			assert.NoError(t, err, "Failed to delete project file")
			err = os.Remove(filepath.Join(dir, "Program.cs"))
			assert.NoError(t, err, "Failed to delete C# project main")
		}()
		err = integration.RunCommand(t, "create dotnet project",
			[]string{ex, "new", "console"}, dir, &integration.ProgramTestOptions{})
		assert.NoError(t, err, "Failed to create C# project")
	}

	// Add dependencies (based on directory name)
	if pkg, pkgVersion := packagesFromTestName(filepath.Base(dir)); pkg != "" {
		err = integration.RunCommand(t, "create dotnet project",
			[]string{ex, "add", "package", pkg, "--version", pkgVersion},
			dir, &integration.ProgramTestOptions{})
		assert.NoError(t, err, "Failed to add dependency %q %q", pkg, pkgVersion)
	} else {
		err = integration.RunCommand(t, "add sdk ref",
			[]string{ex, "add", "reference", "../../../../../../sdk/dotnet/Pulumi"},
			dir, &integration.ProgramTestOptions{})
		assert.NoError(t, err, "Failed to dotnet sdk package reference")
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
	assert.NoError(t, err, "Failed to build dotnet project")
}

// packagesFromName attempts to figure out what package should be imported from
// the name of a test.
//
// Example:
// 	"aws-eks-pp" => ("Pulumi.Aws", 4.21.1)
// 	"azure-sa-pp" => ("Pulumi.Azure", 4.21.1)
// 	"resource-options-pp" => ("","")
//
// Note: While we could instead do this by using the generateMetaData function
// for each language, we are trying not to expand the functionality under test.
func packagesFromTestName(name string) (string, string) {
	if strings.Contains(name, "aws") {
		return "Pulumi.Aws", "4.21.1"
	} else if strings.Contains(name, "azure-native") {
		return "Pulumi.AzureNative", "1.29.0"
	} else if strings.Contains(name, "azure") {
		return "Pulumi.Azure", "4.18.0"
	} else if strings.Contains(name, "kubernetes") {
		return "Pulumi.Kubernetes", "3.7.2"
	} else if strings.Contains(name, "random") {
		return "Pulumi.Random", "4.2.0"
	}
	return "", ""
}
