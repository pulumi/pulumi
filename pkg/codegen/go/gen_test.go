package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
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

func readGoPackageInfo(schemaJsonPath string) (*GoPackageInfo, error) {
	f, err := os.Open(schemaJsonPath)
	if err != nil {
		return nil, err
	}
	type language struct {
		Go GoPackageInfo `json:"go"`
	}
	type model struct {
		Language language `json:"language"`
	}
	var m model
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	return &m.Language.Go, nil
}

// Decide the name of the Go module for a generated test.
//
// For example for this path:
//
// codeDir = "../testing/test/testdata/external-resource-schema/go/"
//
// We will generate "$codeDir/go.mod" using `external-resource-schema` as the module name so that it can compile
// independently.
//
// This can be overwritten by setting ModulePath in GoPackageInfo in
//
//	jq .language.go.modulePath ${codeDir}../schema.json
func inferModuleName(codeDir string) string {
	schemaPath := filepath.Join(filepath.Dir(codeDir), "schema.json")
	if gotSchema, err := test.PathExists(schemaPath); err == nil && gotSchema {
		if info, err := readGoPackageInfo(schemaPath); err == nil {
			if info.ModulePath != "" {
				return info.ModulePath
			}
		}
	}

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
		packages, err := generatePackageContextMap("test", pkg.Reference(), goPkgInfo, nil)
		require.NoError(t, err)

		root, ok := packages[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t)
		}
	})
}

func readSchemaFile(file string) *schema.Package {
	// Read in, decode, and import the schema.
	schemaBytes, err := os.ReadFile(filepath.Join("..", "testing", "test", "testdata", file))
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
			ordering := slice.Prealloc[string](len(files))
			for k := range files {
				ordering = append(ordering, k)
			}
			sort.Strings(ordering)
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
				pkg:            importSpec(t, awsSpec).Reference(),
				importBasePath: awsImportBasePath,
			},
			token:    "aws:s3/BucketWebsite:BucketWebsite",
			expected: "s3.BucketWebsite",
		},
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, awsSpec).Reference(),
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
				pkg:            importSpec(t, googleNativeSpec).Reference(),
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
				pkg:            importSpec(t, awsSpec).Reference(),
				importBasePath: awsImportBasePath,
			},
			token:    "aws:s3/Bucket:Bucket",
			expected: "s3.Bucket",
		},
		{
			pkg: &pkgContext{
				pkg:            importSpec(t, awsSpec).Reference(),
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
				pkg:            importSpec(t, googleNativeSpec).Reference(),
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

func TestGenHeader(t *testing.T) {
	t.Parallel()

	pkg := &pkgContext{
		tool: "a tool",
		pkg:  (&schema.Package{Name: "test-pkg"}).Reference(),
	}

	s := func() string {
		b := &bytes.Buffer{}
		pkg.genHeader(b, []string{"pkg1", "example.com/foo/123-foo"}, nil, false /* isUtil */)
		return b.String()
	}()
	assert.Equal(t, `// Code generated by a tool DO NOT EDIT.
// *** WARNING: Do not edit by hand unless you're certain you know what you are doing! ***

package testpkg

import (
	"pkg1"
	"example.com/foo/123-foo"
)

`, s)

	// Compliance is defined by https://pkg.go.dev/cmd/go#hdr-Generate_Go_files_by_processing_source
	autogenerated := regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)
	found := false
loop:
	for _, l := range strings.Split(s, "\n") {
		switch {
		case autogenerated.Match([]byte(l)):
			found = true
			break loop
		case l == "" || strings.HasPrefix(l, "//"):
		default:
			break loop
		}
	}
	assert.Truef(t, found, `Didn't find a line that complies with "%v"`, autogenerated)
}

func TestTitle(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.Equal("", Title(""))
	assert.Equal("Plugh", Title("plugh"))
	assert.Equal("WaldoThudFred", Title("WaldoThudFred"))
	assert.Equal("WaldoThudFred", Title("waldoThudFred"))
	assert.Equal("WaldoThudFred", Title("waldo-Thud-Fred"))
	assert.Equal("WaldoThudFred", Title("waldo-ThudFred"))
	assert.Equal("WaldoThud_Fred", Title("waldo-Thud_Fred"))
	assert.Equal("WaldoThud_Fred", Title("waldo-thud_Fred"))
}

func TestRegressTypeDuplicatesInChunking(t *testing.T) {
	t.Parallel()
	pkgSpec := schema.PackageSpec{
		Name:      "test",
		Version:   "0.0.1",
		Resources: make(map[string]schema.ResourceSpec),
		Types: map[string]schema.ComplexTypeSpec{
			"test:index:PolicyStatusAutogenRules": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"imageExtractors": {
							TypeSpec: schema.TypeSpec{
								Type: "object",
								AdditionalProperties: &schema.TypeSpec{
									Type: "array",
									Items: &schema.TypeSpec{
										Type: "object",
										Ref:  "#/types/test:index:Im",
									},
								},
							},
						},
					},
				},
			},
			"test:index:Im": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
						"path": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"path"},
				},
			},
		},
	}

	// Need to ref PolicyStatusAutogenRules in input position to trigger the code path.
	pkgSpec.Resources["test:index:Res"] = schema.ResourceSpec{
		InputProperties: map[string]schema.PropertySpec{
			"a": {
				TypeSpec: schema.TypeSpec{
					Ref: "#/types/test:index:PolicyStatusAutogenRules",
				},
			},
		},
	}

	// Need to have N>500 but N<1000 to obtain 2 chunks.
	for i := 0; i < 750; i++ {
		ttok := fmt.Sprintf("test:index:Typ%d", i)
		pkgSpec.Types[ttok] = schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:     "object",
				Required: []string{"x"},
				Properties: map[string]schema.PropertySpec{
					"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
		}
	}

	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))
	pkg, diags, err := schema.BindSpec(pkgSpec, loader)
	require.NoError(t, err)
	t.Logf("%v", diags.Error())
	require.False(t, diags.HasErrors())

	fs, err := GeneratePackage("tests", pkg)
	require.NoError(t, err)

	for f := range fs {
		t.Logf("Generated %v", f)
	}

	// Expect to see two chunked files (chunking at n=500).
	assert.Contains(t, fs, "test/pulumiTypes.go")
	assert.Contains(t, fs, "test/pulumiTypes1.go")
	assert.NotContains(t, fs, "test/pulumiTypes2.go")

	// The types defined in the chunks should be mutually exclusive.
	typedefs := func(s string) []string {
		var types []string
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "type") {
				types = append(types, line)
			}
		}
		sort.Strings(types)
		return types
	}

	typedefs1 := typedefs(string(fs["test/pulumiTypes.go"]))
	typedefs2 := typedefs(string(fs["test/pulumiTypes1.go"]))

	for _, typ := range typedefs1 {
		assert.NotContains(t, typedefs2, typ)
	}

	for _, typ := range typedefs2 {
		assert.NotContains(t, typedefs1, typ)
	}
}
