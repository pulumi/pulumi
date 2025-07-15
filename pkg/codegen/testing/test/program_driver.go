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
	"fmt"
	"io"
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

func transpiled(dir string) string {
	return filepath.Join(transpiledExamplesDir, dir)
}

var allProgLanguages = codegen.NewStringSet(TestDotnet, TestPython, TestGo, TestNodeJS)

type ProgramTest struct {
	Directory          string
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
		if t.Directory == directoryName {
			output = append(output, t)
		}
	}
	return output
}

var PulumiPulumiProgramTests = []ProgramTest{
	{
		Directory:   "assets-archives",
		Description: "Assets and archives",
	},
	{
		Directory:   "synthetic-resource-properties",
		Description: "Synthetic resource properties",
		SkipCompile: codegen.NewStringSet(TestNodeJS, TestDotnet, TestGo), // not a real package
	},
	{
		Directory:   "aws-s3-folder",
		Description: "AWS S3 Folder",
		SkipCompile: codegen.NewStringSet(TestGo),
		// Blocked on go:
		//   TODO[pulumi/pulumi#8064]
		//   TODO[pulumi/pulumi#8065]
	},
	{
		Directory:   "aws-eks",
		Description: "AWS EKS",
	},
	{
		Directory:   "csharp-invoke-options",
		Description: "A program that uses InvokeOptions in C#",
		// Test only C# because the other languages have conformance tests
		Skip: allProgLanguages.Except(TestDotnet),
	},
	{
		Directory:   "aws-fargate",
		Description: "AWS Fargate",
	},
	{
		Directory:   "aws-static-website",
		Description: "an example resource from AWS static website multi-language component",
		// TODO: blocked on resolving imports (python) / using statements (C#) for types from external packages
		SkipCompile: codegen.NewStringSet(TestDotnet, TestPython),
	},
	{
		Directory:   "aws-fargate-output-versioned",
		Description: "AWS Fargate Using Output-versioned invokes for python and typescript",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
		BindOptions: []pcl.BindOption{pcl.PreferOutputVersionedInvokes},
	},
	{
		Directory:   "aws-s3-logging",
		Description: "AWS S3 with logging",
		SkipCompile: codegen.NewStringSet(TestGo),
		// Blocked on nodejs: TODO[pulumi/pulumi#8068]
		// Flaky in go: TODO[pulumi/pulumi#8123]
	},
	{
		Directory:   "aws-iam-policy",
		Description: "AWS IAM Policy",
	},
	{
		Directory:   "read-file-func",
		Description: "ReadFile function translation works",
	},
	{
		Directory:   "python-regress-10914",
		Description: "Python regression test for #10914",
		Skip:        allProgLanguages.Except(TestPython),
	},
	{
		Directory:   "simplified-invokes",
		Description: "Simplified invokes",
		Skip:        codegen.NewStringSet(TestPython, TestGo),
		SkipCompile: codegen.NewStringSet(TestDotnet, TestNodeJS),
	},
	{
		Directory:   "aws-optionals",
		Description: "AWS get invoke with nested object constructor that takes an optional string",
		// Testing Go behavior exclusively:
		Skip: allProgLanguages.Except(TestGo),
	},
	{
		Directory:   "aws-webserver",
		Description: "AWS Webserver",
	},
	{
		Directory:   "simple-range",
		Description: "Simple range as int expression translation",
	},
	{
		Directory:   "azure-native",
		Description: "Azure Native",
		SkipCompile: codegen.NewStringSet(TestGo, TestDotnet),
		// Blocked on go:
		//   TODO[pulumi/pulumi#8073]
		//   TODO[pulumi/pulumi#8074]
	},
	{
		Directory:   "azure-native-v2-eventgrid",
		Description: "Azure Native V2 basic example to ensure that importPathPatten works",
		// Specifically use a simplified azure-native v2.x schema when testing this program
		// this schema only contains content from the eventgrid module which is sufficient to test with
		PluginHost: utils.NewHostWithProviders(testdataPath,
			utils.NewSchemaProvider("azure-native", "2.41.0")),
	},
	{
		Directory:   "azure-sa",
		Description: "Azure SA",
	},
	{
		Directory:   "string-enum-union-list",
		Description: "Contains resource which has a property of type List<Union<String, Enum>>",
		// skipping compiling on Go because it doesn't know to handle unions in lists
		// and instead generates pulumi.StringArray
		SkipCompile: codegen.NewStringSet(TestGo),
	},
	{
		Directory:   "using-object-as-input-for-any",
		Description: "Tests using object as input for a property of type 'any'",
	},
	{
		Directory:   "kubernetes-operator",
		Description: "K8s Operator",
	},
	{
		Directory:   "kubernetes-pod",
		Description: "K8s Pod",
		SkipCompile: codegen.NewStringSet(TestGo),
		// Blocked on go:
		//   TODO[pulumi/pulumi#8073]
		//   TODO[pulumi/pulumi#8074]
	},
	{
		Directory:   "kubernetes-template",
		Description: "K8s Template",
	},
	{
		Directory:   "kubernetes-template-quoted",
		Description: "K8s Template with quoted string property keys to ensure that resource binding works here",
	},
	{
		Directory:   "random-pet",
		Description: "Random Pet",
	},
	{
		Directory:   "aws-secret",
		Description: "Secret",
	},
	{
		Directory:   "functions",
		Description: "Functions",
	},
	{
		Directory:   "output-funcs-aws",
		Description: "Output Versioned Functions",
	},
	{
		Directory:   "third-party-package",
		Description: "Ensuring correct imports for third party packages",
		// compiling and type checking involves downloading the real package to
		// check against. Because we are checking against the "other" package
		// (which doesn't exist), this does not work.
		SkipCompile: codegen.NewStringSet(TestNodeJS, TestDotnet, TestGo),
	},
	{
		Directory: "this-keyword-resource-attr",
		Description: "ensure that the this keyword is rewritten when it is a variable but kept as is" +
			"when it is a reference to this pointer in nodejs",
		Skip: codegen.NewStringSet(TestDotnet, TestPython, TestGo),
	},
	{
		Directory:   "invalid-go-sprintf",
		Description: "Regress invalid Go",
		Skip:        codegen.NewStringSet(TestPython, TestNodeJS, TestDotnet),
	},
	{
		Directory:   "typed-enum",
		Description: "Supply strongly typed enums",
		Skip:        codegen.NewStringSet(TestGo),
	},
	{
		Directory:   "pulumi-stack-reference",
		Description: "StackReference as resource",
	},
	{
		Directory:   "python-resource-names",
		Description: "Repro for #9357",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Directory:   "logical-name",
		Description: "Logical names",
	},
	{
		Directory:   "aws-lambda",
		Description: "AWS Lambdas",
		// We have special testing for this case because lambda is a python keyword.
		Skip: codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Directory:   "basic-unions",
		Description: "Tests program generation of fields of type union",
		SkipCompile: allProgLanguages, // because the schema is synthetic
	},
	{
		Directory:   "deferred-outputs",
		Description: "Tests program with mutually dependant components that emit deferred outputs",
		SkipCompile: allProgLanguages,
	},
	{
		Directory:   "traverse-union-repro",
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
		Directory:   "components",
		Description: "Components",
		SkipCompile: codegen.NewStringSet(TestGo),
	},
	{
		Directory:   "entries-function",
		Description: "Using the entries function",
		// go and dotnet do fully not support GenForExpression yet
		// Todo: https://github.com/pulumi/pulumi/issues/12606
		SkipCompile: allProgLanguages.Except(TestNodeJS).Except(TestPython),
	},
	{
		Directory:   "retain-on-delete",
		Description: "Generate RetainOnDelete option",
	},
	{
		Directory:   "depends-on-array",
		Description: "Using DependsOn resource option with an array of resources",
	},
	{
		Directory:   "multiline-string",
		Description: "Multiline string literals",
	},
	{
		Directory:   "config-variables",
		Description: "Basic program with a bunch of config variables",
		// TODO[https://github.com/pulumi/pulumi/issues/14957] - object config variables are broken here
		SkipCompile: codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Directory:   "regress-11176",
		Description: "Regression test for https://github.com/pulumi/pulumi/issues/11176",
		Skip:        allProgLanguages.Except(TestGo),
	},
	{
		Directory:   "throw-not-implemented",
		Description: "Function notImplemented is compiled to a runtime error at call-site",
	},
	{
		Directory:   "python-reserved",
		Description: "Test python reserved words aren't used",
		Skip:        allProgLanguages.Except(TestPython),
	},
	{
		Directory:   "iterating-optional-range-expressions",
		Description: "Test that we can iterate over range expression that are option(iterator)",
		// TODO: dotnet and go
		Skip: allProgLanguages.Except(TestNodeJS).Except(TestPython),
		// We are using a synthetic schema defined in range-1.0.0.json so we can't compile all the way
		SkipCompile: allProgLanguages,
	},
	{
		Directory:   "output-literals",
		Description: "Tests that we can return various literal values via stack outputs",
	},
	{
		Directory:   "dynamic-entries",
		Description: "Testing iteration of dynamic entries in TypeScript",
		Skip:        allProgLanguages.Except(TestNodeJS),
		SkipCompile: allProgLanguages,
	},
	{
		Directory:   "single-or-none",
		Description: "Tests using the singleOrNone function",
	},
	{
		Directory:   "simple-splat",
		Description: "An example that shows we can compile splat expressions from array of objects",
		// Skip compiling because we are using a test schema without a corresponding real package
		SkipCompile: allProgLanguages,
	},
	{
		Directory:   "invoke-inside-conditional-range",
		Description: "Using the result of an invoke inside a conditional range expression of a resource",
		Skip:        allProgLanguages.Except(TestNodeJS).Except(TestDotnet),
		SkipCompile: allProgLanguages,
	},
	{
		Directory:   "output-name-conflict",
		Description: "Tests whether we are able to generate programs where output variables have same id as config var",
		SkipCompile: codegen.NewStringSet(TestGo),
	},
	{
		Directory:   "snowflake-python-12998",
		Description: "Tests regression for issue https://github.com/pulumi/pulumi/issues/12998",
		Skip:        allProgLanguages.Except(TestPython),
		SkipCompile: allProgLanguages,
		BindOptions: []pcl.BindOption{pcl.AllowMissingVariables, pcl.AllowMissingProperties},
	},
	{
		Directory:   "unknown-resource",
		Description: "Tests generating code for unknown resources when skipping resource type-checking",
		SkipCompile: allProgLanguages,
		BindOptions: []pcl.BindOption{pcl.SkipResourceTypechecking},
	},
	{
		Directory:   "using-dashes",
		Description: "Test program generation on packages with a dash in the name",
		SkipCompile: allProgLanguages, // since we are using a synthetic schema
	},
	{
		Directory:   "unknown-invoke",
		Description: "Tests generating code for unknown invokes when skipping invoke type checking",
		SkipCompile: allProgLanguages,
		BindOptions: []pcl.BindOption{pcl.SkipInvokeTypechecking},
	},
	{
		Directory:   "optional-complex-config",
		Description: "Tests generating code for optional and complex config values",
		Skip:        allProgLanguages.Except(TestNodeJS).Except(TestDotnet),
		SkipCompile: allProgLanguages.Except(TestNodeJS).Except(TestDotnet),
	},
	{
		Directory:   "interpolated-string-keys",
		Description: "Tests that interpolated string keys are supported in maps. ",
		Skip:        allProgLanguages.Except(TestNodeJS).Except(TestPython),
	},
	{
		Directory:   "regress-node-12507",
		Description: "Regression test for https://github.com/pulumi/pulumi/issues/12507",
		Skip:        allProgLanguages.Except(TestNodeJS),
		BindOptions: []pcl.BindOption{pcl.PreferOutputVersionedInvokes},
	},
	{
		Directory:   "csharp-plain-lists",
		Description: "Tests that plain lists are supported in C#",
		Skip:        allProgLanguages.Except(TestDotnet),
	},
	{
		Directory:   "csharp-typed-for-expressions",
		Description: "Testing for expressions with typed target expressions in csharp",
		Skip:        allProgLanguages.Except(TestDotnet),
	},
	{
		Directory:   "empty-list-property",
		Description: "Tests compiling empty list expressions of object properties",
	},
	{
		Directory:   "python-regress-14037",
		Description: "Regression test for rewriting qoutes in python",
		Skip:        allProgLanguages.Except(TestPython),
	},
	{
		Directory:   "inline-invokes",
		Description: "Tests whether using inline invoke expressions works",
		SkipCompile: codegen.NewStringSet(TestGo),
	},
}

var PulumiPulumiYAMLProgramTests = []ProgramTest{
	// PCL files from pulumi/yaml transpiled examples
	{
		Directory:   transpiled("aws-eks"),
		Description: "AWS EKS",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Directory:   transpiled("aws-static-website"),
		Description: "AWS static website",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
		BindOptions: []pcl.BindOption{pcl.SkipResourceTypechecking},
	},
	{
		Directory:   transpiled("awsx-fargate"),
		Description: "AWSx Fargate",
		Skip:        codegen.NewStringSet(TestDotnet, TestNodeJS, TestGo),
	},
	{
		Directory:   transpiled("azure-app-service"),
		Description: "Azure App Service",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Directory:   transpiled("azure-container-apps"),
		Description: "Azure Container Apps",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet, TestPython),
	},
	{
		Directory:   transpiled("azure-static-website"),
		Description: "Azure static website",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet, TestPython),
	},
	{
		Directory:   transpiled("cue-eks"),
		Description: "Cue EKS",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Directory:   transpiled("cue-random"),
		Description: "Cue random",
	},
	{
		Directory:   transpiled("getting-started"),
		Description: "Getting started",
	},
	{
		Directory:   transpiled("kubernetes"),
		Description: "Kubernetes",
		Skip:        codegen.NewStringSet(TestGo),
	},
	{
		Directory:   transpiled("pulumi-variable"),
		Description: "Pulumi variable",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Directory:   transpiled("random"),
		Description: "Random",
		Skip:        codegen.NewStringSet(TestNodeJS),
	},
	{
		Directory:   transpiled("readme"),
		Description: "README",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Directory:   transpiled("stackreference-consumer"),
		Description: "Stack reference consumer",
		Skip:        codegen.NewStringSet(TestGo, TestNodeJS, TestDotnet),
	},
	{
		Directory:   transpiled("stackreference-producer"),
		Description: "Stack reference producer",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet),
	},
	{
		Directory:   transpiled("webserver-json"),
		Description: "Webserver JSON",
		Skip:        codegen.NewStringSet(TestGo, TestDotnet, TestPython),
	},
	{
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

	assert.NotNil(t, testcase.TestCases, "Caller must provide test cases")
	pulumiAccept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
	skipCompile := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_COMPILE_TEST"))

	for _, tt := range testcase.TestCases {
		tt := tt // avoid capturing loop variable
		t.Run(tt.Directory, func(t *testing.T) {
			// These tests should not run in parallel.
			// They take up a fair bit of memory
			// and can OOM in CI with too many running.

			var err error
			if tt.Skip.Has(testcase.Language) {
				t.Skip()
				return
			}

			expectNYIDiags := tt.ExpectNYIDiags.Has(testcase.Language)

			testDir := filepath.Join(testdataPath, tt.Directory+"-pp")
			pclFile := filepath.Join(testDir, tt.Directory+".pp")
			if strings.HasPrefix(tt.Directory, transpiledExamplesDir) {
				pclFile = filepath.Join(testDir, filepath.Base(tt.Directory)+".pp")
			}
			testDir = filepath.Join(testDir, testcase.Language)
			err = os.MkdirAll(testDir, 0o700)
			if err != nil && !os.IsExist(err) {
				t.Fatalf("Failed to create %q: %s", testDir, err)
			}

			contents, err := os.ReadFile(pclFile)
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}

			expectedFile := filepath.Join(testDir, tt.Directory+"."+testcase.Extension)
			if strings.HasPrefix(tt.Directory, transpiledExamplesDir) {
				expectedFile = filepath.Join(testDir, filepath.Base(tt.Directory)+"."+testcase.Extension)
			}
			expected, err := os.ReadFile(expectedFile)
			if err != nil && !pulumiAccept {
				t.Fatalf("could not read %v: %v", expectedFile, err)
			}

			parser := syntax.NewParser()
			err = parser.ParseFile(bytes.NewReader(contents), tt.Directory+".pp")
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}
			if parser.Diagnostics.HasErrors() {
				t.Fatalf("failed to parse files: %v", parser.Diagnostics)
			}

			hclFiles := map[string]*hcl.File{
				tt.Directory + ".pp": {Body: parser.Files[0].Body, Bytes: parser.Files[0].Bytes},
			}
			var pluginHost plugin.Host
			if tt.PluginHost != nil {
				pluginHost = tt.PluginHost
			} else {
				pluginHost = utils.NewHost(testdataPath)
			}

			opts := append(tt.BindOptions, pcl.PluginHost(pluginHost))
			rootProgramPath := filepath.Join(testdataPath, tt.Directory+"-pp")
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
				CheckVersion(t, tt.Directory, depFilePath, testcase.ExpectedVersion)
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
