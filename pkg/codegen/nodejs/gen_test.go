// nolint: lll
package nodejs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "nodejs",
		GenPackage: GeneratePackage,
		Checks: map[string]test.CodegenCheck{
			"nodejs/compile": typeCheckGeneratedPackage,
			"nodejs/test":    testGeneratedPackage,
		},
	})
}

func typeCheckGeneratedPackage(t *testing.T, pwd string) {
	// TODO: previous attempt used npm. It may be more popular and
	// better target than yarn, however our build uses yarn in
	// other places at the moment, and yarn does not run into the
	// ${VERSION} problem; use yarn for now.
	//
	// var npm string
	// npm, err = executable.FindExecutable("npm")
	// require.NoError(t, err)
	// // TODO remove when https://github.com/pulumi/pulumi/pull/7938 lands
	// file := filepath.Join(pwd, "package.json")
	// oldFile, err := ioutil.ReadFile(file)
	// require.NoError(t, err)
	// newFile := strings.ReplaceAll(string(oldFile), "${VERSION}", "0.0.1")
	// err = ioutil.WriteFile(file, []byte(newFile), 0600)
	// require.NoError(t, err)
	// err = integration.RunCommand(t, "npm install", []string{npm, "i"}, pwd, &cmdOptions)
	// require.NoError(t, err)

	test.RunCommand(t, "yarn_link", pwd, "yarn", "link", "@pulumi/pulumi")
	test.RunCommand(t, "yarn_install", pwd, "yarn", "install")
	test.RunCommand(t, "tsc", pwd, "yarn", "run", "tsc", "--noEmit")
}

// Runs unit tests against the generated code.
func testGeneratedPackage(t *testing.T, pwd string) {
	test.RunCommand(t, "mocha", pwd,
		"yarn", "run", "yarn", "run", "mocha", "-r", "ts-node/register", "tests/**/*.spec.ts")
}

func generatePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	p := *pkg
	if len(p.Language) > 0 {
		panic(fmt.Sprintf("%v", p.Language))
	}
	if extraFiles == nil {
		extraFiles = make(map[string][]byte)
	}
	nodePkgInfo := NodePackageInfo{
		PackageName: "@pulumi/mypkg",
		DevDependencies: map[string]string{
			"@types/node":  "latest",
			"@types/mocha": "latest",
			"ts-node":      "latest",
			"mocha":        "latest",
		},
	}
	p.Language["nodejs"] = nodePkgInfo
	return GeneratePackageWithOptions(&GeneratePackageOptions{
		Tool:       tool,
		Pkg:        &p,
		ExtraFiles: extraFiles,
		ExtraFilesInPackageMetadata: []string{
			"tests/codegen.spec.ts",
		},
	})
}

func TestGenerateTypeNames(t *testing.T) {
	test.TestTypeNameCodegen(t, "nodejs", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		modules, info, err := generateModuleContextMap("test", pkg, nil)
		require.NoError(t, err)

		pkg.Language["nodejs"] = info

		root, ok := modules[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t, false, nil)
		}
	})
}
