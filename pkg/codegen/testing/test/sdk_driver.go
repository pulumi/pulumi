// Copyright 2021-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
)

// Defines an extra check logic that accepts the directory with the
// generated code, typically `$TestDir/$test.Directory/$language`.
type CodegenCheck func(t *testing.T, codedir string)

type SDKTest struct {
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

	// Mutex to ensure only a single test operates on directory at a time
	Mutex sync.Mutex
}

// ShouldSkipTest indicates if a given test for a given language should be run.
func (tt *SDKTest) ShouldSkipTest(language, test string) bool {
	// Only language-specific checks.
	if !strings.HasPrefix(test, language+"/") {
		return true
	}

	// Obey SkipCompileCheck to skip compile and test targets.
	if tt.SkipCompileCheck != nil &&
		tt.SkipCompileCheck.Has(language) &&
		(test == language+"/compile" ||
			test == language+"/test") {
		return true
	}

	// Obey Skip.
	if tt.Skip != nil && tt.Skip.Has(test) {
		return true
	}

	return false
}

// ShouldSkipCodegen determines if codegen should be run. ShouldSkipCodegen=true
// further implies no other tests will be run.
func (tt *SDKTest) ShouldSkipCodegen(language string) bool {
	return tt.Skip.Has(language + "/any")
}

var allLanguages = codegen.NewStringSet("python/any", "nodejs/any", "dotnet/any", "go/any", "docs/any")

var PulumiPulumiSDKTests = []*SDKTest{
	{
		Directory:   "naming-collisions",
		Description: "Schema with types that could potentially produce collisions.",
	},
	{
		Directory:   "dash-named-schema",
		Description: "Simple schema with a two part name (foo-bar)",
	},
	{
		Directory:        "external-resource-schema",
		Description:      "External resource schema",
		SkipCompileCheck: codegen.NewStringSet(TestGo),
	},
	{
		Directory:   "nested-module",
		Description: "Nested module",
	},
	{
		Directory:   "simplified-invokes",
		Description: "Simplified invokes",
		Skip:        codegen.NewStringSet("python/any", "go/any"),
	},
	{
		Directory:   "nested-module-thirdparty",
		Description: "Third-party nested module",
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
		Directory:   "simple-schema-pyproject",
		Description: "A simple schema that generates a pyproject.toml file",
		Skip:        codegen.NewStringSet("go/any", "nodejs/any", "dotnet/any"),
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
		SkipCompileCheck: codegen.NewStringSet(TestNodeJS, TestGo),
	},
	{
		Directory:   "simple-methods-schema-single-value-returns",
		Description: "Simple schema with methods that return single values",
	},
	{
		Directory:   "methods-return-plain-resource",
		Description: "Test returning plain Resource objects from methods",
		Skip:        codegen.NewStringSet("dotnet/any"),
	},
	{
		Directory:   "simple-yaml-schema",
		Description: "Simple schema encoded using YAML",
	},
	{
		Directory:   "provider-config-schema",
		Description: "Simple provider config schema",
		// For golang skip check, see https://github.com/pulumi/pulumi/issues/11567
		SkipCompileCheck: codegen.NewStringSet(TestDotnet, TestGo),
	},
	{
		Directory:   "replace-on-change",
		Description: "Simple use of replaceOnChange in schema",
	},
	{
		Directory:   "simple-resource-with-aliases",
		Description: "Simple schema with a resource that has aliases",
	},
	{
		Directory:        "resource-property-overlap",
		Description:      "A resource with the same name as its property",
		SkipCompileCheck: codegen.NewStringSet(TestDotnet, TestNodeJS),
	},
	{
		Directory:   "hyphen-url",
		Description: "A resource url with a hyphen in its path",
		Skip:        codegen.NewStringSet("go/any"),
	},
	{
		Directory:   "output-funcs",
		Description: "Tests targeting the $fn_output helper code generation feature",
	},
	{
		Directory:        "output-funcs-edgeorder",
		Description:      "Regresses Node compilation issues on a subset of azure-native",
		SkipCompileCheck: codegen.NewStringSet(TestGo, TestPython),
		Skip:             codegen.NewStringSet("nodejs/test"),
	},
	{
		Directory:        "output-funcs-tfbridge20",
		Description:      "Similar to output-funcs, but with compatibility: tfbridge20, to simulate pulumi-aws use case",
		SkipCompileCheck: codegen.NewStringSet(TestPython),
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
		Directory:   "plain-and-default",
		Description: "Ensure that a resource with a plain default property works correctly",
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
		SkipCompileCheck: codegen.NewStringSet(TestPython),
	},
	{
		Directory:   "different-package-name-conflict",
		Description: "different packages with the same resource",
		Skip:        allLanguages,
	},
	{
		Directory:   "different-enum",
		Description: "An enum in a different package namespace",
		Skip:        codegen.NewStringSet("dotnet/compile"),
	},
	{
		Directory:   "array-of-enum-map",
		Description: "A schema with an array of maps where the values are enums. Issue pulumi/pulumi#14734",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "azure-native-nested-types",
		Description: "Condensed example of nested collection types from Azure Native",
		Skip:        codegen.NewStringSet("go/any"),
	},
	{
		Directory:   "regress-go-8664",
		Description: "Regress pulumi/pulumi#8664 affecting Go",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "regress-go-10527",
		Description: "Regress pulumi/pulumi#10527 affecting Go",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "regress-go-12971",
		Description: "Regress pulumi/pulumi#12971 affecting Go",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "regress-go-15478",
		Description: "Regress pulumi/pulumi#15478 affecting Go ensuring SDK-gen doesn't panic",
		Skip:        allLanguages.Except("go/any"),
		// skipping the compile step because the generated code emits nested types that are not supported by the Go SDK
		SkipCompileCheck: codegen.NewStringSet(TestGo),
	},
	{
		Directory:   "regress-py-12546",
		Description: "Regress pulumi/pulumi#12546 affecting Python",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "using-shared-types-in-config",
		Description: "Tests that shared types can be used in config.",
	},
	{
		Directory:   "other-owned",
		Description: "CSharp rootNamespaces",
		// We only test in dotnet, because we are testing a change in a dotnet
		// language property. Other tests should pass, but do not put the
		// relevant feature under test. To save time, we skip them.
		//
		// We need to see dotnet changes (paths) in the docs too.
		Skip: allLanguages.Except("dotnet/any").Except("docs/any"),
	},
	{
		Directory: "external-node-compatibility",
		// In this case, this test's schema has kubernetes20 set, but is referencing a type from Google Native
		// which doesn't have any compatibility modes set, so the referenced type should be `AuditConfigArgs`
		// (with the `Args` suffix) and not `AuditConfig`.
		Description: "Ensure external package compatibility modes are used when referencing external types",
		Skip:        allLanguages.Except("nodejs/any"),
	},
	{
		Directory: "external-go-import-aliases",
		// Google Native has its own import aliases, so those should be respected, unless there are local aliases.
		// AWS Classic doesn't have any import aliases, so none should be used, unless there are local aliases.
		Description: "Ensure external import aliases are honored, and any local import aliases override them",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "external-python-same-module-name",
		Description: "Ensure referencing external types/resources with the same module name are referenced correctly",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "enum-reference",
		Description: "Ensure referencing external types/resources with referenced enums import correctly",
	},
	{
		Directory:   "enum-reference-python",
		Description: "Ensure referencing external types/resources with referenced enums import correctly in Python",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "external-enum",
		Description: "Ensure we generate valid tokens for external enums",
		Skip:        codegen.NewStringSet("dotnet/any"),
	},
	{
		Directory:   "internal-dependencies-go",
		Description: "Emit Go internal dependencies",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "go-overridden-internal-module-name",
		Description: "Go SDK where the internal module name is overridden to be 'utilities'",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "go-plain-ref-repro",
		Description: "Generate a resource that accepts a plain input type",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "go-nested-collections",
		Description: "Generate a resource that outputs [][][]Foo",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory: "functions-secrets",
		// Secret properties for non-Output<T> returning functions cannot be secret because they are plain.
		Description: "functions that have properties that are secrets in the schema",
	},
	{
		Directory:        "secrets",
		Description:      "Generate a resource with secret properties",
		SkipCompileCheck: codegen.NewStringSet(TestDotnet),
	},
	{
		Directory:   "regress-py-tfbridge-611",
		Description: "Regresses pulumi/pulumi-terraform-bridge#611",
		Skip:        allLanguages.Except("python/any").Union(codegen.NewStringSet("python/test", "python/py_compile")),
	},
	{
		Directory:   "hyphenated-symbols",
		Description: "Test that types can have names with hyphens in them",
		Skip:        allLanguages.Except("go/any").Except("python/any"),
	},
	{
		Directory:   "provider-type-schema",
		Description: "A schema with a type called Provider schema",
	},
	{
		Directory:   "embedded-crd-types",
		Description: "A schema with CRD types with package names different from the main package",
		Skip:        codegen.NewStringSet("dotnet/any"),
	},
	{
		Directory:   "unions-inside-arrays",
		Description: "A schema with a union type inside an array",
	},
	{
		Directory:   "assets-and-archives",
		Description: "A schema with assets and archives",
	},
	{
		Directory:   "regress-py-14012",
		Description: "Regresses https://github.com/pulumi/pulumi/issues/14012",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "regress-py-14539",
		Description: "Regresses https://github.com/pulumi/pulumi/issues/14539",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "output-funcs-go-generics-only",
		Description: "Tests targeting the $fn_output code generation feature, only for Go generics == generics-only",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "plain-and-default-go-generics-only",
		Description: "resource with a plain default property works, only for Go generics == generics-only",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "secrets-go-generics-only",
		Description: "Generate a resource with secret properties, only for Go generics == generics-only",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "simple-enum-schema-go-generics-only",
		Description: "Simple schema with enum types, only for Go generics == generics-only",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "simple-plain-schema-go-generics-only",
		Description: "Simple schema with plain properties, only for Go generics == generics-only",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "assets-and-archives-go-generics-only",
		Description: "Testing generating a schema with assets and archives for go using generics-only",
		Skip:        allLanguages.Except("go/any"),
	},
	{
		Directory:   "urn-id-properties",
		Description: "Testing urn and id properties in valid locations",
	},
	{
		Directory:   "regress-py-12980",
		Description: "Import resources across modules",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "unions-inline",
		Description: "Testing the use of unions/oneOf in the schema inline with the property definition.",
	},
	{
		Directory:   "legacy-names",
		Description: "Testing the use of snake_case names and tokens.",
		Skip:        codegen.NewStringSet("go/test"),
	},
	{
		Directory:   "kubernetes20",
		Description: "Testing the kubernetes20 compatibility mode.",
		Skip:        codegen.NewStringSet("go/test"),
	},
	{
		Directory:   "python-typed-dict-setuppy",
		Description: "Testing TypedDict generation for inputs.",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "python-typed-dict-disabled-setuppy",
		Description: "Do not generated TypedDict types for inputs.",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "python-typed-dict-pyproject",
		Description: "Testing TypedDict generation for inputs.",
		Skip:        allLanguages.Except("python/any"),
	},
	{
		Directory:   "config-variables",
		Description: "Testing config variables.",
	},
	{
		Directory:   "overlay-supported-languages",
		Description: "Testing restricting the languages an overlay supports.",
		Skip:        allLanguages.Except("docs/any"),
	},
	{
		Directory: "regress-py-17219",
		// The default behavior should be to generate classes and typed dicts.
		Description: "Regress pulumi/pulumi#17219 affecting Python",
		Skip:        allLanguages.Except("python/any"),
	},
}

var genSDKOnly bool

func NoSDKCodegenChecks() bool {
	return genSDKOnly
}

func init() {
	noChecks := false
	if env, ok := os.LookupEnv("PULUMI_TEST_SDK_NO_CHECKS"); ok {
		noChecks, _ = strconv.ParseBool(env)
	}
	flag.BoolVar(&genSDKOnly, "sdk.no-checks", noChecks, "when set, skips all post-SDK-generation checks")

	// NOTE: the testing package will call flag.Parse.
}

// SDKCodegenOptions describes the set of codegen tests for a language.
type SDKCodegenOptions struct {
	// Name of the programming language.
	Language string

	// Language-aware code generator; such as `GeneratePackage`.
	// from `codegen/dotnet`.
	GenPackage GenPkgSignature

	// Extra checks for all the tests. They keys of this map are
	// of the form "$language/$check" such as "go/compile".
	Checks map[string]CodegenCheck

	// The tests to run. A testcase `tt` are assumed to be located at
	// ../testing/test/testdata/${tt.Directory}
	TestCases []*SDKTest
}

// TestSDKCodegen runs the complete set of SDK code generation tests
// against a particular language's code generator. It also verifies
// that the generated code is structurally sound.
//
// The test files live in `pkg/codegen/testing/test/testdata` and
// are registered in `var sdkTests` in `sdk_driver.go`.
//
// An SDK code generation test files consists of a schema and a set of
// expected outputs for each language. Each test is structured as a
// directory that contains that information:
//
//	testdata/
//	    my-simple-schema/   # i.e. `simple-enum-schema`
//	        schema.(json|yaml)
//	        go/
//	        python/
//	        nodejs/
//	        dotnet/
//	        ...
//
// The schema is the only piece that *must* be manually authored.
//
// Once the schema has been written, the actual codegen outputs can be
// generated by running the following in `pkg/codegen` directory:
//
//	PULUMI_ACCEPT=true go test ./...
//
// This will rebuild subfolders such as `go/` from scratch and store
// the set of code-generated file names in `go/codegen-manifest.json`.
// If these outputs look correct, they need to be checked into git and
// will then serve as the expected values for the normal test runs:
//
//	go test ./...
//
// That is, the normal test runs will fail if changes to codegen or
// schema lead to a diff in the generated file set. If the diff is
// intentional, it can be accepted again via `PULUMI_ACCEPT=true`.
//
// To support running unit tests over the generated code, the tests
// also support mixing in manually written `$lang-extras` files into
// the generated tree. For example, given the following input:
//
//	testdata/
//	    my-simple-schema/
//	        schema.json
//	        go/
//	        go-extras/
//	            tests/
//	                go_test.go
//
// The system will copy `go-extras/tests/go_test.go` into
// `go/tests/go_test.go` before performing compilation and unit test
// checks over the project generated in `go`.
func TestSDKCodegen(t *testing.T, opts *SDKCodegenOptions) { // revive:disable-line
	if runtime.GOOS == "windows" {
		t.Skip("TestSDKCodegen is skipped on Windows")
	}

	testDir := filepath.Join("..", "testing", "test", "testdata")

	require.NotNil(t, opts.TestCases, "No test cases were provided. This was probably a mistake")
	for _, tt := range opts.TestCases {
		tt := tt // avoid capturing loop variable `sdkTest` in the closure

		t.Run(tt.Directory, func(t *testing.T) {
			t.Parallel()

			tt.Mutex.Lock()
			t.Cleanup(tt.Mutex.Unlock)

			t.Log(tt.Description)

			dirPath := filepath.Join(testDir, filepath.FromSlash(tt.Directory))

			schemaPath := filepath.Join(dirPath, "schema.json")
			if _, err := os.Stat(schemaPath); err != nil && os.IsNotExist(err) {
				schemaPath = filepath.Join(dirPath, "schema.yaml")
			}

			if tt.ShouldSkipCodegen(opts.Language) {
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

			// Sort the checks in alphabetical order.
			var checkOrder []string
			for check := range allChecks {
				checkOrder = append(checkOrder, check)
			}
			sort.Strings(checkOrder)

			codeDir := filepath.Join(dirPath, opts.Language)

			// Perform the checks.
			//nolint:paralleltest // test functions are ordered
			for _, check := range checkOrder {
				check := check
				t.Run(check, func(t *testing.T) {
					if tt.ShouldSkipTest(opts.Language, check) {
						t.Skip()
					}
					checkFun := allChecks[check]
					checkFun(t, codeDir)
				})
			}
		})
	}
}
