package test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
)

// Defines an extra check logic that accepts the directory with the
// generated code, typically `$TestDir/$test.Directory/$language`.
type CodegenCheck func(t *testing.T, codedir string)

type sdkTest struct {
	Directory   string
	Description string

	// Extra checks for this test. They keys of this map
	// are of the form "$language/$check" such as "go/compile".
	Checks map[string]CodegenCheck

	// Skip checks, identified by "$language/$check".
	Skip codegen.StringSet

	// Do not compile the generated code for the languages in this set.
	// This is a helper form of `Skip`.
	SkipCompileCheck codegen.StringSet
}

const (
	// python = "python"
	nodejs = "nodejs"
	dotnet = "dotnet"
	golang = "go"
)

var sdkTests = []sdkTest{
	{
		Directory:   "naming-collisions",
		Description: "Schema with types that could potentially produce collisions (go).",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "dash-named-schema",
		Description: "Simple schema with a two part name (foo-bar)",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "external-resource-schema",
		Description:      "External resource schema",
		SkipCompileCheck: codegen.NewStringSet(nodejs, golang),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "nested-module",
		Description:      "Nested module",
		SkipCompileCheck: codegen.NewStringSet(dotnet, nodejs),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "nested-module-thirdparty",
		Description:      "Third-party nested module",
		SkipCompileCheck: codegen.NewStringSet(dotnet, nodejs),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "plain-schema-gh6957",
		Description: "Repro for #6957",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "resource-args-python-case-insensitive",
		Description: "Resource args with same named resource and type case insensitive",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "resource-args-python",
		Description: "Resource args with same named resource and type",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "simple-enum-schema",
		Description: "Simple schema with enum types",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "simple-plain-schema",
		Description: "Simple schema with plain properties",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "simple-plain-schema-with-root-package",
		Description: "Simple schema with root package set",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "simple-resource-schema",
		Description: "Simple schema with local resource properties",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "simple-resource-schema-custom-pypackage-name",
		Description: "Simple schema with local resource properties and custom Python package name",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "simple-methods-schema",
		Description:      "Simple schema with methods",
		SkipCompileCheck: codegen.NewStringSet(nodejs, dotnet, golang),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "simple-yaml-schema",
		Description: "Simple schema encoded using YAML",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "provider-config-schema",
		Description:      "Simple provider config schema",
		SkipCompileCheck: codegen.NewStringSet(dotnet),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "replace-on-change",
		Description:      "Simple use of replaceOnChange in schema",
		SkipCompileCheck: codegen.NewStringSet(golang),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "resource-property-overlap",
		Description:      "A resource with the same name as its property",
		SkipCompileCheck: codegen.NewStringSet(dotnet, nodejs),
		Skip:             codegen.NewStringSet("python/test"),
	},
	{
		Directory:   "hyphen-url",
		Description: "A resource url with a hyphen in its path",
		Skip:        codegen.NewStringSet("python/test"),
	},
	{
		Directory:        "output-funcs",
		Description:      "Tests targeting the $fn_output helper code generation feature",
		SkipCompileCheck: codegen.NewStringSet(dotnet),
	},
	{
		Directory:   "cyclic-types",
		Description: "Cyclic object types",
	},
}

var genSDKOnly bool

func init() {
	flag.BoolVar(&genSDKOnly, "sdk.no-checks", false, "when set, skips all post-SDK-generation checks")
	// NOTE: the testing package will call flag.Parse.
}

type SDKCodegenOptions struct {
	// Name of the programming language.
	Language string

	// Language-aware code generator; such as `GeneratePackage`.
	// from `codegen/dotnet`.
	GenPackage GenPkgSignature

	// Extra checks for all the tests. They keys of this map are
	// of the form "$language/$check" such as "go/compile".
	Checks map[string]CodegenCheck
}

// `TestSDKCodegen` runs the complete set of SDK code generation tests
// against a particular language's code generator. It also verifies
// that the generated code is structurally sound.
//
// The tests files live in `pkg/codegen/internal/test/testdata` and
// are registered in `var sdkTests` in `sdk_driver.go`.
//
// An SDK code generation test files consists of a schema and a set of
// expected outputs for each language. Each test is structured as a
// directory that contains that information:
//
//  testdata/
//      my-simple-schema/   # i.e. `simple-enum-schema`
//          schema.(json|yaml)
//          go/
//          python/
//          nodejs/
//          dotnet/
//          ...
//
// The schema is the only piece that *must* be manually authored.
//
// Once the schema has been written, the actual codegen outputs can be
// generated by running the following in `pkg/codegen` directory:
//
//      PULUMI_ACCEPT=true go test ./...
//
// This will rebuild subfolders such as `go/` from scratch and store
// the set of code-generated file names in `go/codegen-manfiest.json`.
// If these outputs look correct, they need to be checked into git and
// will then serve as the expected values for the normal test runs:
//
//      go test ./...
//
// That is, the normal test runs will fail if changes to codegen or
// schema lead to a diff in the generated file set. If the diff is
// intentional, it can be accepted again via `PULUMI_ACCEPT=true`.
//
// To support running unit tests over the generated code, the tests
// also support mixing in manually written `$lang-extras` files into
// the generated tree. For example, given the following input:
//
//  testdata/
//      my-simple-schema/
//          schema.json
//          go/
//          go-extras/
//              tests/
//                  go_test.go
//
// The system will copy `go-extras/tests/go_test.go` into
// `go/tests/go_test.go` before performing compilation and unit test
// checks over the project generated in `go`.
func TestSDKCodegen(t *testing.T, opts *SDKCodegenOptions) { // revive:disable-line
	testDir := filepath.Join("..", "internal", "test", "testdata")

	// Motivation for flagging parallelism: not all tests are
	// parallel-safe yet (for example, codegen/docs tests fail),
	// and there are concerns about memory utilizaion in CI. It
	// can be a nice feature for developing though.
	parallel := os.Getenv("PULUMI_PARALLEL_SDK_CODEGEN_TESTS") != ""

	for _, sdkTest := range sdkTests {
		tt := sdkTest // avoid capturing loop variable `sdkTest` in the closure
		t.Run(tt.Directory, func(t *testing.T) {
			if parallel {
				t.Parallel()
			}

			t.Log(tt.Description)

			dirPath := filepath.Join(testDir, filepath.FromSlash(tt.Directory))

			schemaPath := filepath.Join(dirPath, "schema.json")
			if _, err := os.Stat(schemaPath); err != nil && os.IsNotExist(err) {
				schemaPath = filepath.Join(dirPath, "schema.yaml")
			}

			files, err := GeneratePackageFilesFromSchema(schemaPath, opts.GenPackage)
			require.NoError(t, err)

			if !RewriteFilesWhenPulumiAccept(t, dirPath, opts.Language, files) {
				expectedFiles, err := LoadBaseline(dirPath, opts.Language)
				require.NoError(t, err)

				if !ValidateFileEquality(t, files, expectedFiles) {
					t.Fail()
				}
			}

			if genSDKOnly {
				return
			}

			CopyExtraFiles(t, dirPath, opts.Language)

			// Merge language-specific global and
			// test-specific checks, with test-specific
			// having precedence.
			allChecks := make(map[string]CodegenCheck)
			for k, v := range opts.Checks {
				allChecks[k] = v
			}
			for k, v := range tt.Checks {
				allChecks[k] = v
			}

			// Define check filter.
			shouldSkipCheck := func(check string) bool {

				// Only language-specific checks.
				if !strings.HasPrefix(check, opts.Language+"/") {
					return true
				}

				// Obey SkipCompileCheck to skip compile and test targets.
				if tt.SkipCompileCheck != nil &&
					tt.SkipCompileCheck.Has(opts.Language) &&
					(check == fmt.Sprintf("%s/compile", opts.Language) ||
						check == fmt.Sprintf("%s/test", opts.Language)) {
					return true
				}

				// Obey Skip.
				if tt.Skip != nil && tt.Skip.Has(check) {
					return true
				}

				return false
			}

			// Sort the checks in alphabetical order.
			var checkOrder []string
			for check := range allChecks {
				checkOrder = append(checkOrder, check)
			}
			sort.Strings(checkOrder)

			codeDir := filepath.Join(dirPath, opts.Language)

			// Perform the checks.
			for _, checkVar := range checkOrder {
				check := checkVar
				t.Run(check, func(t *testing.T) {
					if shouldSkipCheck(check) {
						t.Skip()
					}
					checkFun := allChecks[check]
					checkFun(t, codeDir)
				})
			}
		})
	}
}
