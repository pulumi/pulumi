package gen

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func TestInputUsage(t *testing.T) {
	pkg := &pkgContext{}
	arrayUsage := pkg.getInputUsage("FooArray")
	assert.Equal(
		t,
		"FooArrayInput is an input type that accepts FooArray and FooArrayOutput values.\nYou can construct a "+
			"concrete instance of `FooArrayInput` via:\n\n\t\t FooArray{ FooArgs{...} }\n ",
		arrayUsage)

	mapUsage := pkg.getInputUsage("FooMap")
	assert.Equal(
		t,
		"FooMapInput is an input type that accepts FooMap and FooMapOutput values.\nYou can construct a concrete"+
			" instance of `FooMapInput` via:\n\n\t\t FooMap{ \"key\": FooArgs{...} }\n ",
		mapUsage)

	ptrUsage := pkg.getInputUsage("FooPtr")
	assert.Equal(
		t,
		"FooPtrInput is an input type that accepts FooArgs, FooPtr and FooPtrOutput values.\nYou can construct a "+
			"concrete instance of `FooPtrInput` via:\n\n\t\t FooArgs{...}\n\n or:\n\n\t\t nil\n ",
		ptrUsage)

	usage := pkg.getInputUsage("Foo")
	assert.Equal(
		t,
		"FooInput is an input type that accepts FooArgs and FooOutput values.\nYou can construct a concrete instance"+
			" of `FooInput` via:\n\n\t\t FooArgs{...}\n ",
		usage)
}

func TestGoPackageName(t *testing.T) {
	assert.Equal(t, "aws", goPackage("aws"))
	assert.Equal(t, "azure", goPackage("azure-nextgen"))
	assert.Equal(t, "plant", goPackage("plant-provider"))
	assert.Equal(t, "", goPackage(""))
}

func TestGeneratePackage(t *testing.T) {
	generatePackage := func(tool string, pkg *schema.Package, files map[string][]byte) (map[string][]byte, error) {

		for f := range files {
			t.Logf("Ignoring extraFile %s", f)
		}

		return GeneratePackage(tool, pkg)
	}
	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "go",
		GenPackage: generatePackage,
		Checks: map[string]test.CodegenCheck{
			"go/compile": typeCheckGeneratedPackage,
			"go/test":    testGeneratedPackage,
		},
	})
}

func inferModuleName(codeDir string) string {
	// For example for this path:
	//
	// codeDir = "../internal/test/testdata/external-resource-schema/go/"
	//
	// We will generate "$codeDir/go.mod" using
	// `external-resource-schema` as the module name so that it
	// can compile independently.
	return filepath.Base(filepath.Dir(codeDir))
}

func typeCheckGeneratedPackage(t *testing.T, codeDir string) {
	sdk, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk"))
	require.NoError(t, err)

	goExe, err := executable.FindExecutable("go")
	require.NoError(t, err)

	goMod := filepath.Join(codeDir, "go.mod")
	alreadyHaveGoMod, err := test.PathExists(goMod)
	require.NoError(t, err)

	if alreadyHaveGoMod {
		t.Logf("Found an existing go.mod, leaving as is")
	} else {
		test.RunCommand(t, "go_mod_init", codeDir, goExe, "mod", "init", inferModuleName(codeDir))
		replacement := fmt.Sprintf("github.com/pulumi/pulumi/sdk/v3=%s", sdk)
		test.RunCommand(t, "go_mod_edit", codeDir, goExe, "mod", "edit", "-replace", replacement)
	}

	test.RunCommand(t, "go_mod_tidy", codeDir, goExe, "mod", "tidy")
	test.RunCommand(t, "go_build", codeDir, goExe, "build", "-v", "all")
}

func testGeneratedPackage(t *testing.T, codeDir string) {
	goExe, err := executable.FindExecutable("go")
	require.NoError(t, err)

	test.RunCommand(t, "go-test", codeDir, goExe, "test", fmt.Sprintf("%s/...", inferModuleName(codeDir)))
}

func TestGenerateTypeNames(t *testing.T) {
	test.TestTypeNameCodegen(t, "go", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer})
		require.NoError(t, err)

		var goPkgInfo GoPackageInfo
		if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
			goPkgInfo = goInfo
		}
		packages := generatePackageContextMap("test", pkg, goPkgInfo)

		root, ok := packages[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t)
		}
	})
}
