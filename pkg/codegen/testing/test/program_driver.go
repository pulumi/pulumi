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
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	transpiledExamplesDir = "transpiled_examples"
)

func transpiled(dir string) fs.FS { return os.DirFS(filepath.Join(transpiledExamplesDir, dir)) }

var allProgLanguages = codegen.NewStringSet(TestDotnet, TestPython, TestGo, TestNodeJS)

type ProgramTest struct {
	Name               string
	Directory          fs.FS
	Description        string
	Skip               codegen.StringSet
	ExpectNYIDiags     codegen.StringSet
	SkipCompile        codegen.StringSet
	BindOptions        []pcl.BindOption
	MockPluginVersions map[string]string
	PluginHost         plugin.Host
}

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

// Get batch number k (base-1 indexed) of tests out of n batches total.
func ProgramTestBatch(k, n int) []ProgramTest {
	start := ((k - 1) * len(PulumiPulumiProgramTests)) / n
	end := ((k) * len(PulumiPulumiProgramTests)) / n
	return PulumiPulumiProgramTests[start:end]
}

// Useful when debugging a single test case.
func SingleTestCase(directoryName string) []ProgramTest {
	output := make([]ProgramTest, 0)
	for _, t := range PulumiPulumiProgramTests {
		if t.Name == directoryName {
			output = append(output, t)
		}
	}
	return output
}

var PulumiPulumiProgramTests = []ProgramTest{
	{
		Name:        "assets-archives",
		Description: "Assets and archives",
		Directory:   mustTestInputDir("assets-archives"),
	},
	{
		Name:        "synthetic-resource-properties",
		Directory:   mustTestInputDir("synthetic-resource-properties"),
		Description: "Synthetic resource properties",
		SkipCompile: codegen.NewStringSet(TestNodeJS, TestDotnet, TestGo), // not a real package
	},
	{
		Name:        "aws-s3-folder",
		Directory:   mustTestInputDir("aws-s3-folder"),
		Description: "AWS S3 Folder",
		SkipCompile: codegen.NewStringSet(TestGo),
		// Blocked on go:
		//   TODO[pulumi/pulumi#8064]
		//   TODO[pulumi/pulumi#8065]
	},
	{
		Name:        "aws-eks",
		Directory:   mustTestInputDir("aws-eks"),
		Description: "AWS EKS",
	},
	{
		Name:        "csharp-invoke-options",
		Directory:   mustTestInputDir("csharp-invoke-options"),
		Description: "A program that uses InvokeOptions in C#",
		// Test only C# because the other languages have conformance tests
		Skip: allProgLanguages.Except(TestDotnet),
	},
	{
		Name:        "aws-fargate",
		Directory:   mustTestInputDir("aws-fargate"),
		Description: "AWS Fargate",
	},
	{
		Name:        "aws-static-website",
		Directory:   mustTestInputDir("aws-static-website"),
		Description: "an example resource from AWS static website multi-language component",
		// TODO: blocked on resolving imports (python) / using statements (C#) for types from external packages
		SkipCompile: codegen.NewStringSet(TestDotnet, TestPython),
	},
	{
		Name:        "aws-fargate-output-versioned",
		Directory:   mustTestInputDir("aws-fargate-output-versioned"),
		Description: "AWS Fargate Using Output-versioned invokes for python and typescript",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
		BindOptions: []pcl.BindOption{pcl.PreferOutputVersionedInvokes},
	},
	{
		Name:        "aws-s3-logging",
		Directory:   mustTestInputDir("aws-s3-logging"),
		Description: "AWS S3 with logging",
		SkipCompile: codegen.NewStringSet(TestGo),
		// Blocked on nodejs: TODO[pulumi/pulumi#8068]
		// Flaky in go: TODO[pulumi/pulumi#8123]
	},
	{
		Name:        "aws-iam-policy",
		Directory:   mustTestInputDir("aws-iam-policy"),
		Description: "AWS IAM Policy",
	},
	{
		Name:        "read-file-func",
		Directory:   mustTestInputDir("read-file-func"),
		Description: "ReadFile function translation works",
	},
	{
		Name:        "python-regress-10914",
		Directory:   mustTestInputDir("python-regress-10914"),
		Description: "Python regression test for #10914",
		Skip:        allProgLanguages.Except(TestPython),
	},
	{
		Name:        "simplified-invokes",
		Directory:   mustTestInputDir("simplified-invokes"),
		Description: "Simplified invokes",
		Skip:        codegen.NewStringSet(TestPython, TestGo),
		SkipCompile: codegen.NewStringSet(TestDotnet, TestNodeJS),
	},
	{
		Name:        "aws-optionals",
		Directory:   mustTestInputDir("aws-optionals"),
		Description: "AWS get invoke with nested object constructor that takes an optional string",
		// Testing Go behavior exclusively:
		Skip: allProgLanguages.Except(TestGo),
	},
	{
		Name:        "aws-webserver",
		Directory:   mustTestInputDir("aws-webserver"),
		Description: "AWS Webserver",
	},
	{
		Name:        "simple-range",
		Directory:   mustTestInputDir("simple-range"),
		Description: "Simple range as int expression translation",
	},
	{
		Name:        "azure-native",
		Directory:   mustTestInputDir("azure-native"),
		Description: "Azure Native",
		SkipCompile: codegen.NewStringSet(TestGo, TestDotnet),
		// Blocked on go:
		//   TODO[pulumi/pulumi#8073]
		//   TODO[pulumi/pulumi#8074]
	},
	{
		Name:        "azure-native-v2-eventgrid",
		Directory:   mustTestInputDir("azure-native-v2-eventgrid"),
		Description: "Azure Native V2 basic example to ensure that importPathPatten works",
		// Specifically use a simplified azure-native v2.x schema when testing this program
		// this schema only contains content from the eventgrid module which is sufficient to test with
		PluginHost: utils.NewHostWithProviders(testdataPath,
			utils.NewSchemaProvider("azure-native", "2.41.0")),
	},
	{
		Name:        "azure-sa",
		Directory:   mustTestInputDir("azure-sa"),
		Description: "Azure SA",
	},
	{
		Name:        "string-enum-union-list",
		Directory:   mustTestInputDir("string-enum-union-list"),
		Description: "Contains resource which has a property of type List<Union<String, Enum>>",
		// skipping compiling on Go because it doesn't know to handle unions in lists
		// and instead generates pulumi.StringArray
		SkipCompile: codegen.NewStringSet(TestGo),
	},
	{
		Name:        "using-object-as-input-for-any",
		Directory:   mustTestInputDir("using-object-as-input-for-any"),
		Description: "Tests using object as input for a property of type 'any'",
	},
	{
		Name:        "kubernetes-operator",
		Directory:   mustTestInputDir("kubernetes-operator"),
		Description: "K8s Operator",
	},
	{
		Name:        "kubernetes-pod",
		Directory:   mustTestInputDir("kubernetes-pod"),
		Description: "K8s Pod",
		SkipCompile: codegen.NewStringSet(TestGo),
		// Blocked on go:
		//   TODO[pulumi/pulumi#8073]
		//   TODO[pulumi/pulumi#8074]
	},
	{
		Name:        "kubernetes-template",
		Directory:   mustTestInputDir("kubernetes-template"),
		Description: "K8s Template",
	},
	{
		Name:        "kubernetes-template-quoted",
		Directory:   mustTestInputDir("kubernetes-template-quoted"),
		Description: "K8s Template with quoted string property keys to ensure that resource binding works here",
	},
	{
		Name:        "random-pet",
		Directory:   mustTestInputDir("random-pet"),
		Description: "Random Pet",
	},
	{
		Name:        "aws-secret",
		Directory:   mustTestInputDir("aws-secret"),
		Description: "Secret",
	},
	{
		Name:        "functions",
		Directory:   mustTestInputDir("functions"),
		Description: "Functions",
	},
	{
		Name:        "output-funcs-aws",
		Directory:   mustTestInputDir("output-funcs-aws"),
		Description: "Output Versioned Functions",
	},
	{
		Name:        "third-party-package",
		Directory:   mustTestInputDir("third-party-package"),
		Description: "Ensuring correct imports for third party packages",
		// compiling and type checking involves downloading the real package to
		// check against. Because we are checking against the "other" package
		// (which doesn't exist), this does not work.
		SkipCompile: codegen.NewStringSet(TestNodeJS, TestDotnet, TestGo),
	},
	{
		Name:      "this-keyword-resource-attr",
		Directory: mustTestInputDir("this-keyword-resource-attr"),
		Description: "ensure that the this keyword is rewritten when it is a variable but kept as is" +
			"when it is a reference to this pointer in nodejs",
		Skip: codegen.NewStringSet(TestDotnet, TestPython, TestGo),
	},
	{
		Name:        "invalid-go-sprintf",
		Directory:   mustTestInputDir("invalid-go-sprintf"),
		Description: "Regress invalid Go",
		Skip:        codegen.NewStringSet(TestPython, TestNodeJS, TestDotnet),
	},
	{
		Name:        "typed-enum",
		Directory:   mustTestInputDir("typed-enum"),
		Description: "Supply strongly typed enums",
		Skip:        codegen.NewStringSet(TestGo),
	},
	{
		Name:        "pulumi-stack-reference",
		Directory:   mustTestInputDir("pulumi-stack-reference"),
		Description: "StackReference as resource",
	},
	{
		Name:        "python-resource-names",
		Directory:   mustTestInputDir("python-resource-names"),
		Description: "Repro for #9357",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Name:        "logical-name",
		Directory:   mustTestInputDir("logical-name"),
		Description: "Logical names",
	},
	{
		Name:        "aws-lambda",
		Directory:   mustTestInputDir("aws-lambda"),
		Description: "AWS Lambdas",
		// We have special testing for this case because lambda is a python keyword.
		Skip: codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Name:        "basic-unions",
		Directory:   mustTestInputDir("basic-unions"),
		Description: "Tests program generation of fields of type union",
		SkipCompile: allProgLanguages, // because the schema is synthetic
	},
	{
		Name:        "deferred-outputs",
		Directory:   mustTestInputDir("deferred-outputs"),
		Description: "Tests program with mutually dependant components that emit deferred outputs",
		SkipCompile: allProgLanguages,
	},
	{
		Name:        "traverse-union-repro",
		Directory:   mustTestInputDir("traverse-union-repro"),
		Description: `Handling the error "cannot traverse value of type union(...)"`,
		BindOptions: []pcl.BindOption{
			pcl.SkipResourceTypechecking,
			pcl.AllowMissingVariables,
			pcl.AllowMissingProperties,
		},
		// The example is known to be invalid. Specifically it hands a
		// `[aws_subnet.test1.id]` to a `string` attribute, where `aws_subnet` is not in
		// scope.
		//
		// The example is invalid in two ways:
		// 1. `aws_subnet` is a missing variable.
		// 2. `[...]` is a tuple, which can never be a string.
		//
		// Even though the generated code will not type check, it should still be
		// directionally correct.
		SkipCompile: allProgLanguages,
	},
	{
		Name:        "components",
		Directory:   mustTestInputDir("components"),
		Description: "Components",
		SkipCompile: codegen.NewStringSet(TestGo),
	},
	{
		Name:        "entries-function",
		Directory:   mustTestInputDir("entries-function"),
		Description: "Using the entries function",
		// go and dotnet do fully not support GenForExpression yet
		// Todo: https://github.com/pulumi/pulumi/issues/12606
		SkipCompile: allProgLanguages.Except(TestNodeJS).Except(TestPython),
	},
	{
		Name:        "retain-on-delete",
		Directory:   mustTestInputDir("retain-on-delete"),
		Description: "Generate RetainOnDelete option",
	},
	{
		Name:        "depends-on-array",
		Directory:   mustTestInputDir("depends-on-array"),
		Description: "Using DependsOn resource option with an array of resources",
	},
	{
		Name:        "multiline-string",
		Directory:   mustTestInputDir("multiline-string"),
		Description: "Multiline string literals",
	},
	{
		Name:        "config-variables",
		Directory:   mustTestInputDir("config-variables"),
		Description: "Basic program with a bunch of config variables",
		// TODO[https://github.com/pulumi/pulumi/issues/14957] - object config variables are broken here
		SkipCompile: codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Name:        "regress-11176",
		Directory:   mustTestInputDir("regress-11176"),
		Description: "Regression test for https://github.com/pulumi/pulumi/issues/11176",
		Skip:        allProgLanguages.Except(TestGo),
	},
	{
		Name:        "throw-not-implemented",
		Directory:   mustTestInputDir("throw-not-implemented"),
		Description: "Function notImplemented is compiled to a runtime error at call-site",
	},
	{
		Name:        "python-reserved",
		Directory:   mustTestInputDir("python-reserved"),
		Description: "Test python reserved words aren't used",
		Skip:        allProgLanguages.Except(TestPython),
	},
	{
		Name:        "iterating-optional-range-expressions",
		Directory:   mustTestInputDir("iterating-optional-range-expressions"),
		Description: "Test that we can iterate over range expression that are option(iterator)",
		// TODO: dotnet and go
		Skip: allProgLanguages.Except(TestNodeJS).Except(TestPython),
		// We are using a synthetic schema defined in range-1.0.0.json so we can't compile all the way
		SkipCompile: allProgLanguages,
	},
	{
		Name:        "output-literals",
		Directory:   mustTestInputDir("output-literals"),
		Description: "Tests that we can return various literal values via stack outputs",
	},
	{
		Name:        "dynamic-entries",
		Directory:   mustTestInputDir("dynamic-entries"),
		Description: "Testing iteration of dynamic entries in TypeScript",
		Skip:        allProgLanguages.Except(TestNodeJS),
		SkipCompile: allProgLanguages,
	},
	{
		Name:        "single-or-none",
		Directory:   mustTestInputDir("single-or-none"),
		Description: "Tests using the singleOrNone function",
	},
	{
		Name:        "simple-splat",
		Directory:   mustTestInputDir("simple-splat"),
		Description: "An example that shows we can compile splat expressions from array of objects",
		// Skip compiling because we are using a test schema without a corresponding real package
		SkipCompile: allProgLanguages,
	},
	{
		Name:        "invoke-inside-conditional-range",
		Directory:   mustTestInputDir("invoke-inside-conditional-range"),
		Description: "Using the result of an invoke inside a conditional range expression of a resource",
		Skip:        allProgLanguages.Except(TestNodeJS).Except(TestDotnet),
		SkipCompile: allProgLanguages,
	},
	{
		Name:        "output-name-conflict",
		Directory:   mustTestInputDir("output-name-conflict"),
		Description: "Tests whether we are able to generate programs where output variables have same id as config var",
		SkipCompile: codegen.NewStringSet(TestGo),
	},
	{
		Name:        "snowflake-python-12998",
		Directory:   mustTestInputDir("snowflake-python-12998"),
		Description: "Tests regression for issue https://github.com/pulumi/pulumi/issues/12998",
		Skip:        allProgLanguages.Except(TestPython),
		SkipCompile: allProgLanguages,
		BindOptions: []pcl.BindOption{pcl.AllowMissingVariables, pcl.AllowMissingProperties},
	},
	{
		Name:        "unknown-resource",
		Directory:   mustTestInputDir("unknown-resource"),
		Description: "Tests generating code for unknown resources when skipping resource type-checking",
		SkipCompile: allProgLanguages,
		BindOptions: []pcl.BindOption{pcl.SkipResourceTypechecking},
	},
	{
		Name:        "using-dashes",
		Directory:   mustTestInputDir("using-dashes"),
		Description: "Test program generation on packages with a dash in the name",
		SkipCompile: allProgLanguages, // since we are using a synthetic schema
	},
	{
		Name:        "unknown-invoke",
		Directory:   mustTestInputDir("unknown-invoke"),
		Description: "Tests generating code for unknown invokes when skipping invoke type checking",
		SkipCompile: allProgLanguages,
		BindOptions: []pcl.BindOption{pcl.SkipInvokeTypechecking},
	},
	{
		Name:        "optional-complex-config",
		Directory:   mustTestInputDir("optional-complex-config"),
		Description: "Tests generating code for optional and complex config values",
		Skip:        allProgLanguages.Except(TestNodeJS).Except(TestDotnet),
		SkipCompile: allProgLanguages.Except(TestNodeJS).Except(TestDotnet),
	},
	{
		Name:        "interpolated-string-keys",
		Directory:   mustTestInputDir("interpolated-string-keys"),
		Description: "Tests that interpolated string keys are supported in maps. ",
		Skip:        allProgLanguages.Except(TestNodeJS).Except(TestPython),
	},
	{
		Name:        "regress-node-12507",
		Directory:   mustTestInputDir("regress-node-12507"),
		Description: "Regression test for https://github.com/pulumi/pulumi/issues/12507",
		Skip:        allProgLanguages.Except(TestNodeJS),
		BindOptions: []pcl.BindOption{pcl.PreferOutputVersionedInvokes},
	},
	{
		Name:        "csharp-plain-lists",
		Directory:   mustTestInputDir("csharp-plain-lists"),
		Description: "Tests that plain lists are supported in C#",
		Skip:        allProgLanguages.Except(TestDotnet),
	},
	{
		Name:        "csharp-typed-for-expressions",
		Directory:   mustTestInputDir("csharp-typed-for-expressions"),
		Description: "Testing for expressions with typed target expressions in csharp",
		Skip:        allProgLanguages.Except(TestDotnet),
	},
	{
		Name:        "empty-list-property",
		Directory:   mustTestInputDir("empty-list-property"),
		Description: "Tests compiling empty list expressions of object properties",
	},
	{
		Name:        "python-regress-14037",
		Directory:   mustTestInputDir("python-regress-14037"),
		Description: "Regression test for rewriting qoutes in python",
		Skip:        allProgLanguages.Except(TestPython),
	},
	{
		Name:        "inline-invokes",
		Directory:   mustTestInputDir("inline-invokes"),
		Description: "Tests whether using inline invoke expressions works",
		SkipCompile: codegen.NewStringSet(TestGo),
	},
}

var PulumiPulumiYAMLProgramTests = []ProgramTest{
	// PCL files from pulumi/yaml transpiled examples
	{
		Name:        "aws-eks",
		Directory:   transpiled("aws-eks"),
		Description: "AWS EKS",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Name:        "aws-static-website",
		Directory:   transpiled("aws-static-website"),
		Description: "AWS static website",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
		BindOptions: []pcl.BindOption{pcl.SkipResourceTypechecking},
	},
	{
		Name:        "awsx-fargate",
		Directory:   transpiled("awsx-fargate"),
		Description: "AWSx Fargate",
		Skip:        codegen.NewStringSet(TestDotnet, TestNodeJS, TestGo),
	},
	{
		Name:        "azure-app-service",
		Directory:   transpiled("azure-app-service"),
		Description: "Azure App Service",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Name:        "azure-container-apps",
		Directory:   transpiled("azure-container-apps"),
		Description: "Azure Container Apps",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet, TestPython),
	},
	{
		Name:        "azure-static-website",
		Directory:   transpiled("azure-static-website"),
		Description: "Azure static website",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet, TestPython),
	},
	{
		Name:        "cue-eks",
		Directory:   transpiled("cue-eks"),
		Description: "Cue EKS",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Name:        "cue-random",
		Directory:   transpiled("cue-random"),
		Description: "Cue random",
	},
	{
		Name:        "getting-started",
		Directory:   transpiled("getting-started"),
		Description: "Getting started",
	},
	{
		Name:        "kubernetes",
		Directory:   transpiled("kubernetes"),
		Description: "Kubernetes",
		Skip:        codegen.NewStringSet(TestGo),
	},
	{
		Name:        "pulumi-variable",
		Directory:   transpiled("pulumi-variable"),
		Description: "Pulumi variable",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Name:        "random",
		Directory:   transpiled("random"),
		Description: "Random",
		Skip:        codegen.NewStringSet(TestNodeJS),
	},
	{
		Name:        "readme",
		Directory:   transpiled("readme"),
		Description: "README",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Name:        "stackreference-consumer",
		Directory:   transpiled("stackreference-consumer"),
		Description: "Stack reference consumer",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Name:        "stackreference-producer",
		Directory:   transpiled("stackreference-producer"),
		Description: "Stack reference producer",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Name:        "webserver-json",
		Directory:   transpiled("webserver-json"),
		Description: "Webserver JSON",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet, TestPython),
	},
	{
		Name:        "webserver",
		Directory:   transpiled("webserver"),
		Description: "Webserver",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet, TestPython),
	},
}

// Checks that a generated program is correct
//
// The arguments are to be read:
// (Testing environment, path to generated code, set of dependencies)
type CheckProgramOutput = func(*testing.T, string, codegen.StringSet)

// Generates a program from a pcl.Program
type GenProgram = func(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error)

// Generates a project from a pcl.Program
type GenProject = func(
	directory string, project workspace.Project,
	program *pcl.Program, localDependencies map[string]string,
) error

type ProgramCodegenOptions struct {
	Language   string
	Extension  string
	OutputFile string
	Check      CheckProgramOutput
	GenProgram GenProgram
	TestCases  []ProgramTest

	// For generating a full project
	IsGenProject bool
	GenProject   GenProject
	// Maps a test file (i.e. "aws-resource-options") to a struct containing a package
	// (i.e. "github.com/pulumi/pulumi-aws/sdk/v5", "pulumi-aws) and its
	// version prefixed by an operator (i.e. " v5.11.0", ==5.11.0")
	ExpectedVersion map[string]PkgVersionInfo
	DependencyFile  string
}

type PkgVersionInfo struct {
	Pkg          string
	OpAndVersion string
}

//go:embed testinputs
var testinputs embed.FS

func mustTestInputDir(subdir string) fs.FS {
	sub, err := fs.Sub(testinputs, subdir)
	if err != nil {
		panic(err)
	}
	return sub
}

// TestProgramCodegen runs the complete set of program code generation tests against a particular
// language's code generator.
//
// A program code generation test consists of a PCL file (.pp extension) and a set of expected outputs
// for each language.
//
// The PCL file is the only piece that must be manually authored. Once the schema has been written, the expected outputs
// can be generated by running `PULUMI_ACCEPT=true go test ./..." from the `pkg/codegen` directory.
//
//nolint:revive
func TestProgramCodegen(
	t *testing.T,
	testcase ProgramCodegenOptions,
) {
	if runtime.GOOS == "windows" {
		t.Skip("TestProgramCodegen is skipped on Windows")
	}

	require.NotNil(t, testcase.TestCases, "Caller must provide test cases")
	pulumiAccept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
	skipCompile := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_COMPILE_TEST"))

	for _, tt := range testcase.TestCases {
		t.Run(tt.Name, func(t *testing.T) {
			// These tests should not run in parallel.
			// They take up a fair bit of memory
			// and can OOM in CI with too many running.

			var err error
			if tt.Skip.Has(testcase.Language) {
				t.Skip()
				return
			}

			expectNYIDiags := tt.ExpectNYIDiags.Has(testcase.Language)

			testDir := filepath.Join(testdataPath, tt.Name+"-pp")
			pclFile, err := tt.Directory.Open("main.pp")
			require.NoError(t, err)
			testDir = filepath.Join(testDir, testcase.Language)
			err = os.MkdirAll(testDir, 0o700)
			if err != nil && !os.IsExist(err) {
				t.Fatalf("Failed to create %q: %s", testDir, err)
			}

			expectedFile := filepath.Join(testDir, tt.Name+"."+testcase.Extension)
			expected, err := os.ReadFile(expectedFile)
			if err != nil && !pulumiAccept {
				t.Fatalf("could not read %v: %v", expectedFile, err)
			}

			parser := syntax.NewParser()
			err = parser.ParseFile(pclFile, tt.Name+".pp")
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}
			if parser.Diagnostics.HasErrors() {
				t.Fatalf("failed to parse files: %v", parser.Diagnostics)
			}

			hclFiles := map[string]*hcl.File{
				tt.Name + ".pp": {Body: parser.Files[0].Body, Bytes: parser.Files[0].Bytes},
			}
			var pluginHost plugin.Host
			if tt.PluginHost != nil {
				pluginHost = tt.PluginHost
			} else {
				pluginHost = utils.NewHost(testdataPath)
			}

			opts := append(tt.BindOptions, pcl.PluginHost(pluginHost))
			rootProgramPath := filepath.Join(testdataPath, tt.Name+"-pp")
			absoluteProgramPath, err := filepath.Abs(rootProgramPath)
			if err != nil {
				t.Fatalf("failed to bind program: unable to find the absolute path of %v", rootProgramPath)
			}
			opts = append(opts, pcl.DirPath(absoluteProgramPath))
			opts = append(opts, pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

			program, diags, err := pcl.BindProgram(parser.Files, opts...)
			if err != nil {
				t.Fatalf("could not bind program: %v", err)
			}
			bindDiags := new(bytes.Buffer)
			if len(diags) > 0 {
				require.NoError(t, hcl.NewDiagnosticTextWriter(bindDiags, hclFiles, 80, false).WriteDiagnostics(diags))
				if diags.HasErrors() {
					t.Fatalf("failed to bind program:\n%s", bindDiags)
				}
				t.Logf("bind diags:\n%s", bindDiags)
			}
			var files map[string][]byte
			// generate a full project and check expected package versions
			if testcase.IsGenProject {
				project := workspace.Project{
					Name:    "test",
					Runtime: workspace.NewProjectRuntimeInfo(testcase.Language, nil),
				}
				err = testcase.GenProject(testDir, project, program, nil /*localDependencies*/)
				require.NoError(t, err)

				depFilePath := filepath.Join(testDir, testcase.DependencyFile)
				outfilePath := filepath.Join(testDir, testcase.OutputFile)
				CheckVersion(t, tt.Name, depFilePath, testcase.ExpectedVersion)
				GenProjectCleanUp(t, testDir, depFilePath, outfilePath)
			}
			files, diags, err = testcase.GenProgram(program)
			require.NoError(t, err)
			if expectNYIDiags {
				var tmpDiags hcl.Diagnostics
				for _, d := range diags {
					if !strings.HasPrefix(d.Summary, "not yet implemented") {
						tmpDiags = append(tmpDiags, d)
					}
				}
				diags = tmpDiags
			}
			if diags.HasErrors() {
				buf := new(bytes.Buffer)

				err := hcl.NewDiagnosticTextWriter(buf, hclFiles, 80, false).WriteDiagnostics(diags)
				require.NoError(t, err, "Failed to write diag")

				t.Fatalf("failed to generate program:\n%s", buf)
			}

			if pulumiAccept {
				err := os.WriteFile(expectedFile, files[testcase.OutputFile], 0o600)
				require.NoError(t, err)
				// generate the rest of the files
				for fileName, content := range files {
					if fileName != testcase.OutputFile {
						outputPath := filepath.Join(testDir, fileName)
						err := os.WriteFile(outputPath, content, 0o600)
						require.NoError(t, err, "Failed to write file %s", outputPath)
					}
				}
			} else {
				assert.Equal(t, string(expected), string(files[testcase.OutputFile]))
				// assert that the content is correct for the rest of the files
				for fileName, content := range files {
					if fileName != testcase.OutputFile {
						outputPath := filepath.Join(testDir, fileName)
						outputContent, err := os.ReadFile(outputPath)
						require.NoError(t, err)
						assert.Equal(t, string(outputContent), string(content))
					}
				}
			}
			if !skipCompile && testcase.Check != nil && !tt.SkipCompile.Has(testcase.Language) {
				extraPulumiPackages := codegen.NewStringSet()
				collectExtraPulumiPackages(program, extraPulumiPackages)
				testcase.Check(t, expectedFile, extraPulumiPackages)
			}
		})
	}
}

func collectExtraPulumiPackages(program *pcl.Program, extraPulumiPackages codegen.StringSet) {
	for _, n := range program.Nodes {
		if r, isResource := n.(*pcl.Resource); isResource {
			pkg, _, _, _ := r.DecomposeToken()
			if pkg != "pulumi" {
				extraPulumiPackages.Add(pkg)
			}
		}

		if component, isComponent := n.(*pcl.Component); isComponent {
			collectExtraPulumiPackages(component.Program, extraPulumiPackages)
		}
	}
}

// CheckVersion checks for an expected package version
// Todo: support checking multiple package expected versions
func CheckVersion(t *testing.T, dir, depFilePath string, expectedVersionMap map[string]PkgVersionInfo) {
	depFile, err := os.Open(depFilePath)
	require.NoError(t, err)
	defer depFile.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(depFile)

	match := false
	expectedPkg, expectedVersion := strings.TrimSpace(expectedVersionMap[dir].Pkg),
		strings.TrimSpace(expectedVersionMap[dir].OpAndVersion)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, expectedPkg) {
			line = strings.TrimSpace(line)
			actualVersion := strings.TrimPrefix(line, expectedPkg)
			actualVersion = strings.TrimSpace(actualVersion)
			expectedVersion = strings.Trim(expectedVersion, "v:^/> ")
			actualVersion = strings.Trim(actualVersion, "v:^/> ")
			if expectedVersion == actualVersion {
				match = true
				break
			}
			actualSemver, err := semver.Make(actualVersion)
			if err == nil {
				continue
			}
			expectedSemver, _ := semver.Make(expectedVersion)
			if actualSemver.Compare(expectedSemver) >= 0 {
				match = true
				break
			}
		}
	}
	require.Truef(t, match, "Did not find expected package version pair (%q,%q). Searched in:\n%s",
		expectedPkg, expectedVersion, newLazyStringer(func() string {
			// Reset the read on the file
			_, err := depFile.Seek(0, io.SeekStart)
			require.NoError(t, err)
			buf := new(strings.Builder)
			_, err = io.Copy(buf, depFile)
			require.NoError(t, err)
			return buf.String()
		}).String())
}

func GenProjectCleanUp(t *testing.T, dir, depFilePath, outfilePath string) {
	os.Remove(depFilePath)
	os.Remove(outfilePath)
	os.Remove(dir + "/.gitignore")
	os.Remove(dir + "/Pulumi.yaml")
}

type lazyStringer struct {
	cache string
	f     func() string
}

func (l lazyStringer) String() string {
	if l.cache == "" {
		l.cache = l.f()
	}
	return l.cache
}

// The `fmt` `%s` calls .String() if the object is not a string itself. We can delay
// computing expensive display logic until and unless we actually will use it.
func newLazyStringer(f func() string) fmt.Stringer {
	return lazyStringer{f: f}
}
