package nodejs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func Check(t *testing.T, path string, dependencies codegen.StringSet, linkLocal bool) {
	dir := filepath.Dir(path)

	removeFile := func(name string) {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); !os.IsNotExist(err) {
			require.NoError(t, err, "Failed to delete '%s'", path)
		}
	}

	// We delete and regenerate package files for each run.
	removeFile("yarn.lock")
	removeFile("package.json")
	removeFile("tsconfig.json")

	// Write out package.json
	pkgs := nodejsPackages(t, dependencies)
	pkgInfo := npmPackage{
		Dependencies: map[string]string{
			"@pulumi/pulumi": "latest",
		},
		DevDependencies: map[string]string{
			"@types/node": "^17.0.14",
			"typescript":  "^4.5.5",
		},
	}
	for pkg, v := range pkgs {
		pkgInfo.Dependencies[pkg] = v
	}
	pkgJSON, err := json.MarshalIndent(pkgInfo, "", "    ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "package.json"), pkgJSON, 0o600)
	require.NoError(t, err)

	tsConfig := map[string]string{}
	tsConfigJSON, err := json.MarshalIndent(tsConfig, "", "    ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "tsconfig.json"), tsConfigJSON, 0o600)
	require.NoError(t, err)

	TypeCheck(t, path, dependencies, linkLocal)
}

func TypeCheck(t *testing.T, path string, _ codegen.StringSet, linkLocal bool) {
	dir := filepath.Dir(path)

	typeCheckGeneratedPackage(t, dir, linkLocal)
}

func typeCheckGeneratedPackage(t *testing.T, pwd string, linkLocal bool) {
	// NOTE: previous attempt used npm. It may be more popular and
	// better target than yarn, however our build uses yarn in
	// other places at the moment, and yarn does not run into the
	// ${VERSION} problem; use yarn for now.

	if linkLocal {
		test.RunCommand(t, "yarn_link", pwd, "yarn", "link", "@pulumi/pulumi")
	}
	test.RunCommand(t, "yarn_install", pwd, "yarn", "install")
	tscOptions := &integration.ProgramTestOptions{
		// Avoid Out of Memory error on CI:
		Env: []string{"NODE_OPTIONS=--max_old_space_size=4096"},
	}
	test.RunCommandWithOptions(t, tscOptions, "tsc", pwd, "yarn", "run", "tsc",
		"--noEmit", "--skipLibCheck", "true", "--skipDefaultLibCheck", "true")
}

// Returns the nodejs equivalent to the hcl2 package names provided.
func nodejsPackages(t *testing.T, deps codegen.StringSet) map[string]string {
	result := make(map[string]string, len(deps))
	for _, d := range deps.SortedValues() {
		pkgName := "@pulumi/" + d
		set := func(pkgVersion string) {
			result[pkgName] = "^" + pkgVersion
		}
		switch d {
		case "aws":
			set(test.AwsSchema)
		case "azure-native":
			set(test.AzureNativeSchema)
		case "azure":
			set(test.AzureSchema)
		case "kubernetes":
			set(test.KubernetesSchema)
		case "random":
			set(test.RandomSchema)
		case "eks":
			set(test.EksSchema)
		default:
			t.Logf("Unknown package requested: %s", d)
		}

	}
	return result
}

func GenerateProgramBatchTest(t *testing.T, testCases []test.ProgramTest) {
	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, true)
			},
			GenProgram: GenerateProgram,
			TestCases:  testCases,
		})
}
