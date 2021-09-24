package test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/utils"
)

type programTest struct {
	Name           string
	Description    string
	Skip           codegen.StringSet
	ExpectNYIDiags codegen.StringSet
	SkipCompile    codegen.StringSet
}

var testdataPath = filepath.Join("..", "internal", "test", "testdata")

var programTests = []programTest{
	{
		Name:           "aws-s3-folder",
		Description:    "AWS S3 Folder",
		ExpectNYIDiags: codegen.NewStringSet("python", "nodejs", "dotnet"),
		SkipCompile:    codegen.NewStringSet("go", "python", "nodejs"),
		// Blocked on nodejs:
		// Program is invalid syntactically and semantically. This starts with
		// Line 3: import * from "fs"; which should be import * as fs from "fs";
	},
	{
		Name:        "aws-eks",
		Description: "AWS EKS",
		SkipCompile: codegen.NewStringSet("go", "nodejs"),
		// Blocked on go:
		// https://github.com/pulumi/pulumi-aws/issues/1632
		//
		// Blocked on nodejs
		// Starting with:
		// aws-eks.ts:34:65 - error TS1005: ';' expected.
		//
		// 34     for (const range of zones.names.map((k, v) => {key: k, value: v})) {
		//                                                                    ~
	},
	{
		Name:        "aws-fargate",
		Description: "AWS Fargate",
		SkipCompile: codegen.NewStringSet("go"),
		// Blocked on go:
		// https://github.com/pulumi/pulumi-aws/issues/1632
	},
	{
		Name:        "aws-s3-logging",
		Description: "AWS S3 with logging",
		SkipCompile: codegen.NewStringSet("dotnet", "nodejs"),
		// Blocked on dotnet:
		// /codegen/internal/test/testdata/aws-s3-logging-pp/aws-s3-logging.cs(21,71):
		// error CS0023: Operator '?' cannot be applied to operand of type 'ImmutableArray<BucketLogging>'
		//
		// Blocked on nodejs:
		// It looks like this is being parsed as a ternary expression
		// aws-s3-logging.ts:8:89 - error TS1005: ':' expected.
		//
		// 8: export const targetBucket = bucket.loggings.apply(loggings => loggings?[0]?.targetBucket);
		//                                                                                            ~
	},
	{
		Name:        "aws-webserver",
		Description: "AWS Webserver",
		SkipCompile: codegen.NewStringSet("go"),
		// Blocked on go:
		// https://github.com/pulumi/pulumi-aws/issues/1632
	},
	{
		Name:        "azure-native",
		Description: "Azure Native",
		Skip:        codegen.NewStringSet("go", "nodejs"),
		// Blocked on go:
		// Blocked on nodjs:
		// Types do not line up
	},
	{
		Name:        "azure-sa",
		Description: "Azure SA",
	},
	{
		Name:        "kubernetes-operator",
		Description: "K8s Operator",
	},
	{
		Name:        "kubernetes-pod",
		Description: "K8s Pod",
		SkipCompile: codegen.NewStringSet("go", "nodejs"),
		// Blocked on go:
		// Blocked on nodejs:
		// Types do not line up
	},
	{
		Name:        "kubernetes-template",
		Description: "K8s Template",
	},
	{
		Name:        "random-pet",
		Description: "Random Pet",
	},
	{
		Name:        "aws-resource-options",
		Description: "Resource Options",
		SkipCompile: codegen.NewStringSet("go"),
		// Blocked on go:
		// generating invalid aws.Provider code
	},
	{
		Name:        "aws-secret",
		Description: "Secret",
	},
	{
		Name:        "functions",
		Description: "Functions",
		SkipCompile: codegen.NewStringSet("go", "dotnet"),
		// Blocked on go:
		// # main
		// ./functions.go:12:5: no new variables on left side of :=
		// ./functions.go:13:5: no new variables on left side of :=
		//
		// Blocked on dotnet:
		// testdata/functions-pp/functions.cs(9,38): error CS1525: Invalid expression term '{' [functions-pp.csproj]
		// testdata/functions-pp/functions.cs(9,38): error CS1026: ) expected [testdata/functions-pp/functions-pp.csproj]
		// testdata/functions-pp/functions.cs(9,38): error CS1002: ; expected [testdata/functions-pp/functions-pp.csproj]
		// testdata/functions-pp/functions.cs(11,19): error CS1002: ; expected [testdata/functions-pp/functions-pp.csproj]
		// testdata/functions-pp/functions.cs(11,19): error CS1513: } expected [testdata/functions-pp/functions-pp.csproj]
		// testdata/functions-pp/functions.cs(12,23): error CS1002: ; expected [testdata/functions-pp/functions-pp.csproj]
		// testdata/functions-pp/functions.cs(12,23): error CS1513: } expected [testdata/functions-pp/functions-pp.csproj]
		// testdata/functions-pp/functions.cs(13,10): error CS1513: } expected [testdata/functions-pp/functions-pp.csproj]
		// 0 Warning(s)
		// 8 Error(s)

	},
}

// Checks that a generated program is correct
type CheckProgramOutput = func(*testing.T, string)

// Generates a program from a hcl2.Program
type GenProgram = func(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error)

type ProgramLangConfig struct {
	Language   string
	Extension  string
	OutputFile string
	Check      CheckProgramOutput
	GenProgram GenProgram
}

// TestProgramCodegen runs the complete set of program code generation tests against a particular
// language's code generator.
//
// A program code generation test consists of a PCL file (.pp extension) and a set of expected outputs
// for each language.
//
// The PCL file is the only piece that must be manually authored. Once the schema has been written, the expected outputs
// can be generated by running `PULUMI_ACCEPT=true go test ./..." from the `pkg/codegen` directory.
//nolint: revive
func TestProgramCodegen(
	t *testing.T,
	// language string,
	// genProgram func(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error
	testcase ProgramLangConfig,

) {
	ensureValidSchemaVersions(t)
	for _, tt := range programTests {
		t.Run(tt.Description, func(t *testing.T) {
			var err error
			if tt.Skip.Has(testcase.Language) {
				t.Skip()
				return
			}

			expectNYIDiags := tt.ExpectNYIDiags.Has(testcase.Language)

			testDir := filepath.Join(testdataPath, tt.Name+"-pp")
			err = os.Mkdir(testDir, 0700)
			if err != nil && !os.IsExist(err) {
				t.Fatalf("Failed to create %q: %s", testDir, err)
			}

			pclFile := filepath.Join(testDir, tt.Name+".pp")
			contents, err := ioutil.ReadFile(pclFile)
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}

			expectedFile := filepath.Join(testDir, tt.Name+"."+testcase.Extension)
			expected, err := ioutil.ReadFile(expectedFile)
			if err != nil && os.Getenv("PULUMI_ACCEPT") == "" {
				t.Fatalf("could not read %v: %v", expectedFile, err)
			}

			parser := syntax.NewParser()
			err = parser.ParseFile(bytes.NewReader(contents), tt.Name+".pp")
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}
			if parser.Diagnostics.HasErrors() {
				t.Fatalf("failed to parse files: %v", parser.Diagnostics)
			}

			program, diags, err := hcl2.BindProgram(parser.Files, hcl2.PluginHost(utils.NewHost(testdataPath)))
			if err != nil {
				t.Fatalf("could not bind program: %v", err)
			}
			if diags.HasErrors() {
				t.Fatalf("failed to bind program: %v", diags)
			}
			files, diags, err := testcase.GenProgram(program)
			assert.NoError(t, err)
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
				t.Fatalf("failed to generate program: %v", diags)
			}

			if os.Getenv("PULUMI_ACCEPT") != "" {
				err := ioutil.WriteFile(expectedFile, files[testcase.OutputFile], 0600)
				require.NoError(t, err)
			} else {
				assert.Equal(t, string(expected), string(files[testcase.OutputFile]))
			}
			if testcase.Check != nil && !tt.SkipCompile.Has(testcase.Language) {
				testcase.Check(t, expectedFile)
			}
		})
	}
}
