package nodejs

import (
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
	test.TestProgramCodegen(t, test.ProgramLangConfig{
		Language:   "nodejs",
		Extension:  "ts",
		OutputFile: "index.ts",
		Check:      nodejsCheck,
		GenProgram: GenerateProgram,
	})
}

func nodejsCheck(t *testing.T, path string) {
	ex, err := executable.FindExecutable("yarn")
	assert.NoError(t, err, "Could not find yarn executable")
	dir := filepath.Dir(path)
	name := filepath.Base(dir)
	pkgName, pkgVersion := packagesFromTestName(name)
	if pkgName == "" {
		pkgName = "@pulumi/pulumi"
		pkgVersion = "3.7.0"
	}
	pkg := pkgName + "@" + pkgVersion
	defer func() {
		nodeModules := filepath.Join(dir, "node_modules")
		err = os.RemoveAll(nodeModules)
		assert.NoError(t, err, "Failed to delete %s", nodeModules)
		packageJSON := filepath.Join(dir, "package.json")
		err = os.Remove(packageJSON)
		assert.NoError(t, err, "Failed to delete %s", packageJSON)
		yarnLock := filepath.Join(dir, "yarn.lock")
		err = os.Remove(yarnLock)
		assert.NoError(t, err, "Failed to delete %s", yarnLock)
	}()
	err = integration.RunCommand(t, "link @pulumi/pulumi",
		[]string{ex, "link", "@pulumi/pulumi"},
		dir, &integration.ProgramTestOptions{})
	assert.NoError(t, err, "Failed to link @pulumi/pulumi")
	err = integration.RunCommand(t, "yarn add and install",
		[]string{ex, "add", pkg}, dir, &integration.ProgramTestOptions{})
	assert.NoError(t, err, "Could not install package: %q", pkg)
	err = integration.RunCommand(t, "tsc check",
		[]string{ex, "run", "tsc", "--noEmit", filepath.Base(path)}, dir, &integration.ProgramTestOptions{})
	assert.NoError(t, err, "Failed to build %q", path)
}

// packagesFromTestName attempts to figure out what package should be imported
// from the name of the test.
func packagesFromTestName(name string) (string, string) {
	if strings.Contains(name, "aws") {
		return "@pulumi/aws", test.AwsSchema
	} else if strings.Contains(name, "azure-native") {
		return "@pulumi/azure-native", test.AzureNativeSchema
	} else if strings.Contains(name, "azure") {
		return "@pulumi/azure", test.AzureSchema
	} else if strings.Contains(name, "kubernetes") {
		return "@pulumi/kubernetes", test.KubernetesSchema
	} else if strings.Contains(name, "random") {
		return "@pulumi/random", test.RandomSchema
	}
	return "", ""
}
