package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
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
	test.TestSDKCodegen(t, &test.TestSDKCodegenOptions{
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
	goExe, err := executable.FindExecutable("go")
	require.NoError(t, err)

	// Remove existing `go.mod` first otherise `go mod init` fails.
	goMod := filepath.Join(codeDir, "go.mod")
	alreadyHaveGoMod, err := test.PathExists(goMod)
	require.NoError(t, err)
	if alreadyHaveGoMod {
		err := os.Remove(goMod)
		require.NoError(t, err)
	}

	runCommand(t, "go_mod_init", codeDir, goExe, "mod", "init", inferModuleName(codeDir))
	runCommand(t, "go_mod_tidy", codeDir, goExe, "mod", "tidy")
	runCommand(t, "go_build", codeDir, goExe, "build", "-v", "all")
}

func testGeneratedPackage(t *testing.T, codeDir string) {
	goExe, err := executable.FindExecutable("go")
	require.NoError(t, err)

	runCommand(t, "go-test", codeDir, goExe, "test", fmt.Sprintf("%s/...", inferModuleName(codeDir)))
}

func runCommand(t *testing.T, name string, cwd string, executable string, args ...string) {
	wd, err := filepath.Abs(cwd)
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	cmdOptions := integration.ProgramTestOptions{Stderr: &stderr, Stdout: &stdout, Verbose: true}
	err = integration.RunCommand(t,
		name,
		append([]string{executable}, args...),
		wd,
		&cmdOptions)
	require.NoError(t, err)
	if err != nil {
		stdout := stdout.String()
		stderr := stderr.String()
		if len(stdout) > 0 {
			t.Logf("stdout: %s", stdout)
		}
		if len(stderr) > 0 {
			t.Logf("stderr: %s", stderr)
		}
		t.FailNow()
	}
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

func TestGenerateOutputFuncs(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	var examples []string
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".json") {
			examples = append(examples, strings.TrimSuffix(name, ".json"))
		}
	}

	sort.Slice(examples, func(i, j int) bool { return examples[i] < examples[j] })

	gen := func(reader io.Reader, writer io.Writer) error {
		var pkgSpec schema.PackageSpec
		err := json.NewDecoder(reader).Decode(&pkgSpec)
		if err != nil {
			return err
		}
		pkg, err := schema.ImportSpec(pkgSpec, nil)
		if err != nil {
			return err
		}

		tool := "tool"
		var goPkgInfo GoPackageInfo
		if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
			goPkgInfo = goInfo
		}
		pkgContexts := generatePackageContextMap(tool, pkg, goPkgInfo)

		var pkgContext *pkgContext

		for _, c := range pkgContexts {
			if len(c.functionNames) == 1 {
				pkgContext = c
			}
		}

		if pkgContext == nil {
			return fmt.Errorf("Cannot find a package with 1 function in generatePackageContextMap result")
		}

		fun := pkg.Functions[0]
		_, err = writer.Write([]byte(pkgContext.genFunctionCodeFile(fun)))
		return err

	}

	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			inputFile := filepath.Join(testDir, fmt.Sprintf("%s.json", ex))
			expectedOutputFile := filepath.Join(testDir, "go", fmt.Sprintf("%s.go", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)
		})
	}

	goDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs", "go")

	t.Run("compileGeneratedCode", func(t *testing.T) {
		t.Logf("cd %s && go mod tidy", goDir)
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = goDir
		assert.NoError(t, cmd.Run())

		t.Logf("cd %s && go build .", goDir)
		cmd = exec.Command("go", "build", ".")
		cmd.Dir = goDir
		assert.NoError(t, cmd.Run())
	})

	t.Run("testGeneratedCode", func(t *testing.T) {
		t.Logf("cd %s && go test .", goDir)
		cmd := exec.Command("go", "test", ".")
		cmd.Dir = goDir
		assert.NoError(t, cmd.Run())
	})
}
