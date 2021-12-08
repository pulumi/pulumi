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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
	// "$language/any" is special, skipping generating the
	// code as well as all tests.
	Skip codegen.StringSet

	// Do not compile the generated code for the languages in this set.
	// This is a helper form of `Skip`.
	SkipCompileCheck codegen.StringSet
}

const (
	python = "python"
	nodejs = "nodejs"
	dotnet = "dotnet"
	golang = "go"
)

var sdkTests = []sdkTest{
	{
		Directory:   "naming-collisions",
		Description: "Schema with types that could potentially produce collisions (go).",
	},
	{
		Directory:   "dash-named-schema",
		Description: "Simple schema with a two part name (foo-bar)",
	},
	{
		Directory:        "external-resource-schema",
		Description:      "External resource schema",
		SkipCompileCheck: codegen.NewStringSet(nodejs, golang),
	},
	{
		Directory:        "nested-module",
		Description:      "Nested module",
		SkipCompileCheck: codegen.NewStringSet(dotnet, nodejs),
	},
	{
		Directory:        "nested-module-thirdparty",
		Description:      "Third-party nested module",
		SkipCompileCheck: codegen.NewStringSet(dotnet, nodejs),
	},
	{
		Directory:   "plain-schema-gh6957",
		Description: "Repro for #6957",
	},
	{
		Directory:   "resource-args-python-case-insensitive",
		Description: "Resource args with same named resource and type case insensitive",
	},
	{
		Directory:   "resource-args-python",
		Description: "Resource args with same named resource and type",
	},
	{
		Directory:   "simple-enum-schema",
		Description: "Simple schema with enum types",
	},
	{
		Directory:   "simple-plain-schema",
		Description: "Simple schema with plain properties",
	},
	{
		Directory:   "simple-plain-schema-with-root-package",
		Description: "Simple schema with root package set",
	},
	{
		Directory:   "simple-resource-schema",
		Description: "Simple schema with local resource properties",
	},
	{
		Directory:   "simple-resource-schema-custom-pypackage-name",
		Description: "Simple schema with local resource properties and custom Python package name",
	},
	{
		Directory:        "simple-methods-schema",
		Description:      "Simple schema with methods",
		SkipCompileCheck: codegen.NewStringSet(nodejs, golang),
	},
	{
		Directory:   "simple-methods-schema-single-value-returns",
		Description: "Simple schema with methods that return single values",
	},
	{
		Directory:   "simple-yaml-schema",
		Description: "Simple schema encoded using YAML",
	},
	{
		Directory:        "provider-config-schema",
		Description:      "Simple provider config schema",
		SkipCompileCheck: codegen.NewStringSet(dotnet),
	},
	{
		Directory:        "replace-on-change",
		Description:      "Simple use of replaceOnChange in schema",
		SkipCompileCheck: codegen.NewStringSet(golang),
	},
	{
		Directory:        "resource-property-overlap",
		Description:      "A resource with the same name as its property",
		SkipCompileCheck: codegen.NewStringSet(dotnet, nodejs),
	},
	{
		Directory:   "hyphen-url",
		Description: "A resource url with a hyphen in its path",
	},
	{
		Directory:   "output-funcs",
		Description: "Tests targeting the $fn_output helper code generation feature",
	},
	{
		Directory:        "output-funcs-edgeorder",
		Description:      "Regresses Node compilation issues on a subset of azure-native",
		SkipCompileCheck: codegen.NewStringSet(golang, python),
		Skip:             codegen.NewStringSet("nodejs/test"),
	},
	{
		Directory:        "output-funcs-tfbridge20",
		Description:      "Similar to output-funcs, but with compatibility: tfbridge20, to simulate pulumi-aws use case",
		SkipCompileCheck: codegen.NewStringSet(python),
	},
	{
		Directory:   "cyclic-types",
		Description: "Cyclic object types",
	},
	{
		Directory:   "regress-node-8110",
		Description: "Test the fix for pulumi/pulumi#8110 nodejs compilation error",
		Skip:        codegen.NewStringSet("go/test", "dotnet/test"),
	},
	{
		Directory:   "dashed-import-schema",
		Description: "Ensure that we handle all valid go import paths",
		Skip:        codegen.NewStringSet("go/test", "dotnet/test"),
	},
	{
		Directory:        "plain-and-default",
		Description:      "Ensure that a resource with a plain default property works correctly",
		SkipCompileCheck: codegen.NewStringSet(nodejs),
	},
	{
		Directory:   "plain-object-defaults",
		Description: "Ensure that object defaults are generated (repro #8132)",
	},
	{
		Directory:   "plain-object-disable-defaults",
		Description: "Ensure that we can still compile safely when defaults are disabled",
	},
	{
		Directory:        "regress-8403",
		Description:      "Regress pulumi/pulumi#8403",
		SkipCompileCheck: codegen.NewStringSet(python, nodejs),
	},
	{
		Directory:   "different-package-name-conflict",
		Description: "different packages with the same resource",
		Skip:        codegen.NewStringSet("dotnet/any", "nodejs/any", "python/any", "go/any", "docs/any"),
	},
	{
		Directory:   "different-enum",
		Description: "An enum in a different package namespace",
		Skip:        codegen.NewStringSet("dotnet/compile"),
	},
}

var genSDKOnly bool

func NoSDKCodegenChecks() bool {
	return genSDKOnly
}

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

// TestSDKCodegen runs the complete set of SDK code generation tests
// against a particular language's code generator. It also verifies
// that the generated code is structurally sound.
//
// The test files live in `pkg/codegen/internal/test/testdata` and
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
// the set of code-generated file names in `go/codegen-manifest.json`.
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

	// Motivation for flagging: concerns about memory utilizaion
	// in CI. It can be a nice feature for developing though.
	parallel := cmdutil.IsTruthy(os.Getenv("PULUMI_PARALLEL_SDK_CODEGEN_TESTS"))

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

			// Any takes place before codegen.
			if tt.Skip.Has(opts.Language + "/any") {
				t.Logf("Skipping generation + tests for %s", tt.Directory)
				return
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
