// nolint: lll
package nodejs

import (
	// "encoding/json"
	// "fmt"
	// "io"
	// "io/ioutil"
	// "os/exec"
	"fmt"
	"path/filepath"
	// "sort"
	// "strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "nodejs", GeneratePackage)
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

// Resolves modules via Yarn and compiles the code with TypeScript
// linking against in-repo Pulumi Node SDK.
func compileHook() test.SdkTestHook {
	return test.SdkTestHook{
		Name: "compile",
		RunHook: func(env *test.SdkTestEnv) {
			env.Command("yarn", "install")
			env.Command("yarn", "link", "@pulumi/pulumi")
			tsc := env.NewCommand("tsc")
			tsc.Env = []string{fmt.Sprintf("PATH=%s", filepath.Join(".", "node_modules", ".bin"))}
			env.RunCommand(tsc)
		},
	}
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
			"@types/mocha": "^2.2.42",
			"mocha":        "^3.5.0",
		},
	}
	p.Language["nodejs"] = nodePkgInfo
	return GeneratePackageWithOptions(&GeneratePackageOptions{
		Tool:       tool,
		Pkg:        &p,
		ExtraFiles: extraFiles,
		ExtraFilesInPackageMetadata: []string{
			"codegenTests.ts",
		},
	})
}

func TestGenerateOutputFuncsNode(t *testing.T) {
	testRootDir := filepath.Join("..", "internal", "test", "testdata")

	test.TestSDKCodegenWithOptions(t, &test.TestSDKCodegenOptions{
		Language:    "nodejs",
		GenPackage:  generatePackage,
		TestRootDir: testRootDir,
		SDKTests: []test.SdkTest{
			{
				Directory:   "output-funcs",
				Description: "output-funcs",
				IncludeLanguage: func(lang string) bool {
					return lang == "nodejs"
				},
				HooksByLanguage: map[string][]test.SdkTestHook{
					"nodejs": {
						compileHook(),
					},
				},
			},
		},
	})

	// TODO run unit tests
}
