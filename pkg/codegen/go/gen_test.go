package gen

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func TestInputUsage(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	assert.Equal(t, "aws", goPackage("aws"))
	assert.Equal(t, "azurenextgen", goPackage("azure-nextgen"))
	assert.Equal(t, "plantprovider", goPackage("plant-provider"))
	assert.Equal(t, "", goPackage(""))
}

func TestGeneratePackage(t *testing.T) {
	t.Parallel()

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
		TestCases: test.PulumiPulumiSDKTests,
	})
}

func inferModuleName(codeDir string) string {
	// For example for this path:
	//
	// codeDir = "../testing/test/testdata/external-resource-schema/go/"
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
	t.Parallel()

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

func readSchemaFile(file string) *schema.Package {
	// Read in, decode, and import the schema.
	schemaBytes, err := ioutil.ReadFile(filepath.Join("..", "testing", "test", "testdata", file))
	if err != nil {
		panic(err)
	}
	var pkgSpec schema.PackageSpec
	if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
		panic(err)
	}
	pkg, err := schema.ImportSpec(pkgSpec, map[string]schema.Language{"go": Importer})
	if err != nil {
		panic(err)
	}

	return pkg
}

// We test the naming/module structure of generated packages.
func TestPackageNaming(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		importBasePath  string
		rootPackageName string
		name            string
		expectedRoot    string
	}{
		{
			importBasePath: "github.com/pulumi/pulumi-azure-quickstart-acr-geo-replication/sdk/go/acr",
			expectedRoot:   "acr",
		},
		{
			importBasePath:  "github.com/ihave/animport",
			rootPackageName: "root",
			expectedRoot:    "",
		},
		{
			name:         "named-package",
			expectedRoot: "namedpackage",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.expectedRoot, func(t *testing.T) {
			t.Parallel()

			// This schema is arbitrary. We just needed a filled out schema. All
			// path decisions should be made based off of the Name and
			// Language[go] fields (which we set after import).
			schema := readSchemaFile(filepath.Join("schema", "good-enum-1.json"))
			if tt.name != "" {
				// We want there to be a name, so if one isn't provided we
				// default to the schema.
				schema.Name = tt.name
			}
			schema.Language = map[string]interface{}{
				"go": GoPackageInfo{
					ImportBasePath:  tt.importBasePath,
					RootPackageName: tt.rootPackageName,
				},
			}
			files, err := GeneratePackage("test", schema)
			require.NoError(t, err)
			ordering := make([]string, len(files))
			var i int
			for k := range files {
				ordering[i] = k
				i++
			}
			ordering = sort.StringSlice(ordering)
			require.NotEmpty(t, files, "This test only works when files are generated")
			for _, k := range ordering {
				root := strings.Split(k, "/")[0]
				if tt.expectedRoot != "" {
					require.Equal(t, tt.expectedRoot, root, "Root should precede all cases. Got file %s", k)
				}
				// We should work on a way to assert this is one level higher then it otherwise would be.
			}
		})
	}
}

func TestTokenToType(t *testing.T) {
	t.Parallel()

	const awsImportBasePath = "github.com/pulumi/pulumi-aws/sdk/v4/go/aws"
	awsSpec := schema.PackageSpec{
		Name: "aws",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
	}

	const googleNativeImportBasePath = "github.com/pulumi/pulumi-google-native/sdk/go/google"
	googleNativeSpec := schema.PackageSpec{
		Name: "google-native",
	}

	tests := []struct {
		pkg      *pkgContext
		token    string
		expected string
	}{
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, awsSpec),
				importBasePath: awsImportBasePath,
			},
			token:    "aws:s3/BucketWebsite:BucketWebsite",
			expected: "s3.BucketWebsite",
		},
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, awsSpec),
				importBasePath: awsImportBasePath,
				pkgImportAliases: map[string]string{
					"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3": "awss3",
				},
			},
			token:    "aws:s3/BucketWebsite:BucketWebsite",
			expected: "awss3.BucketWebsite",
		},
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, googleNativeSpec),
				importBasePath: googleNativeImportBasePath,
				pkgImportAliases: map[string]string{
					"github.com/pulumi/pulumi-google-native/sdk/go/google/dns/v1": "dns",
				},
			},
			token:    "google-native:dns/v1:DnsKeySpec",
			expected: "dns.DnsKeySpec",
		},
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, tt := range tests {
		tt := tt
		t.Run(tt.token+"=>"+tt.expected, func(t *testing.T) {
			t.Parallel()

			actual := tt.pkg.tokenToType(tt.token)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestTokenToResource(t *testing.T) {
	t.Parallel()

	const awsImportBasePath = "github.com/pulumi/pulumi-aws/sdk/v4/go/aws"
	awsSpec := schema.PackageSpec{
		Name: "aws",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
	}

	const googleNativeImportBasePath = "github.com/pulumi/pulumi-google-native/sdk/go/google"
	googleNativeSpec := schema.PackageSpec{
		Name: "google-native",
	}

	tests := []struct {
		pkg      *pkgContext
		token    string
		expected string
	}{
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, awsSpec),
				importBasePath: awsImportBasePath,
			},
			token:    "aws:s3/Bucket:Bucket",
			expected: "s3.Bucket",
		},
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, awsSpec),
				importBasePath: awsImportBasePath,
				pkgImportAliases: map[string]string{
					"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3": "awss3",
				},
			},
			token:    "aws:s3/Bucket:Bucket",
			expected: "awss3.Bucket",
		},
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, googleNativeSpec),
				importBasePath: googleNativeImportBasePath,
				pkgImportAliases: map[string]string{
					"github.com/pulumi/pulumi-google-native/sdk/go/google/dns/v1": "dns",
				},
			},
			token:    "google-native:dns/v1:Policy",
			expected: "dns.Policy",
		},
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, tt := range tests {
		tt := tt
		t.Run(tt.token+"=>"+tt.expected, func(t *testing.T) {
			t.Parallel()

			actual := tt.pkg.tokenToResource(tt.token)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func importSpec(t *testing.T, spec schema.PackageSpec) *schema.Package {
	importedPkg, err := schema.ImportSpec(spec, map[string]schema.Language{})
	assert.NoError(t, err)
	return importedPkg
}
