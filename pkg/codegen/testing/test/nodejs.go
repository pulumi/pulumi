package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"
)

func generateNodeBatchTest(t *testing.T, generator GenProgram, testCases []ProgramTest) {
	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				CheckNodeJS(t, path, dependencies, true)
			},
			GenProgram: generator,
			TestCases:  testCases,
		})
}

func CheckNodeJS(t *testing.T, path string, dependencies codegen.StringSet, linkLocal bool) {
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
	pkgInfo := pkg{
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

	typeCheckNode(t, path, dependencies, linkLocal)
}

func typeCheckNode(t *testing.T, path string, _ codegen.StringSet, linkLocal bool) {
	dir := filepath.Dir(path)

	TypeCheckNodeJSPackage(t, dir, linkLocal)
}

func TypeCheckNodeJSPackage(t *testing.T, pwd string, linkLocal bool) {
	RunCommand(t, "npm_install", pwd, "npm", "install")
	if linkLocal {
		RunCommand(t, "yarn_link", pwd, "yarn", "link", "@pulumi/pulumi")
	}
	tscOptions := &integration.ProgramTestOptions{
		// Avoid Out of Memory error on CI:
		Env: []string{"NODE_OPTIONS=--max_old_space_size=4096"},
	}
	RunCommandWithOptions(t, tscOptions, "tsc", pwd, filepath.Join(pwd, "node_modules", ".bin", "tsc"),
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
			set(AwsSchema)
		case "azure-native":
			set(AzureNativeSchema)
		case "azure":
			set(AzureSchema)
		case "kubernetes":
			set(KubernetesSchema)
		case "random":
			set(RandomSchema)
		case "eks":
			set(EksSchema)
		case "aws-static-website":
			set(AwsStaticWebsiteSchema)
		case "aws-native":
			set(AwsNativeSchema)
		default:
			t.Logf("Unknown package requested: %s", d)
		}

	}
	return result
}

type pkg struct {
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}
