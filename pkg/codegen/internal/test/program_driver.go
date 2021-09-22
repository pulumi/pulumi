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
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/python"
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

var langConfig = map[string]struct {
	extension  string
	outputFile string
	// Will be called on the generated file
	check func(*testing.T, string)
}{
	"python": {
		extension:  "py",
		outputFile: "__main__.py",
		check: func(t *testing.T, path string) {
			ex, _, err := python.CommandPath()
			assert.NoError(t, err)
			name := filepath.Base(path)
			dir := filepath.Dir(path)
			err = integration.RunCommand(t, "python syntax check",
				[]string{ex, "-m", "py_compile", name}, dir, &integration.ProgramTestOptions{})
			assert.NoError(t, err)
		},
	},
	"nodejs": {
		extension:  "ts",
		outputFile: "index.ts",
		check: func(t *testing.T, path string) {
			ex, err := executable.FindExecutable("yarn")
			assert.NoError(t, err, "Could not find yarn executable")
			dir := filepath.Dir(path)
			name := filepath.Base(dir)
			pkgName, pkgVersion := packagesFromNameNodejs(name)
			if pkgName == "" {
				pkgName = "@pulumi/pulumi"
				pkgVersion = "3.7.0"
			}
			pkg := pkgName + "@" + pkgVersion
			defer func() {
				nodeModules := filepath.Join(dir, "node_modules")
				err = os.RemoveAll(nodeModules)
				assert.NoError(t, err, "Failed to delete %s", nodeModules)
				packageJSON := filepath.Join(dir, "package.json")
				err = os.Remove(packageJSON)
				assert.NoError(t, err, "Failed to delete %s", packageJSON)
				yarnLock := filepath.Join(dir, "yarn.lock")
				err = os.Remove(yarnLock)
				assert.NoError(t, err, "Failed to delete %s", yarnLock)
			}()
			err = integration.RunCommand(t, "yarn add and install",
				[]string{ex, "add", pkg}, dir, &integration.ProgramTestOptions{})
			assert.NoError(t, err, "Could not install package: %q", pkg)
			err = integration.RunCommand(t, "tsc check",
				[]string{ex, "run", "tsc", "--noEmit", filepath.Base(path)}, dir, &integration.ProgramTestOptions{})
			assert.NoError(t, err, "Failed to build %q", path)
		},
	},
	"go": {
		extension:  "go",
		outputFile: "main.go",
		check: func(t *testing.T, path string) {
			dir := filepath.Dir(path)
			ex, err := executable.FindExecutable("go")
			assert.NoError(t, err)
			_, err = ioutil.ReadFile("go.mod")
			if os.IsNotExist(err) {
				defer func() {
					// If we created the module, we also remove the module
					err = os.Remove(filepath.Join(dir, "go.mod"))
					assert.NoError(t, err)
					err = os.Remove(filepath.Join(dir, "go.sum"))
					assert.NoError(t, err)
				}()
				err = integration.RunCommand(t, "generate go.mod",
					[]string{ex, "mod", "init", "main"},
					dir, &integration.ProgramTestOptions{})
				assert.NoError(t, err)
				err = integration.RunCommand(t, "go tidy",
					[]string{ex, "mod", "tidy"},
					dir, &integration.ProgramTestOptions{})
				assert.NoError(t, err)
			} else {
				assert.NoError(t, err)
			}
			err = integration.RunCommand(t, "test build", []string{ex, "build"},
				dir, &integration.ProgramTestOptions{})
			assert.NoError(t, err)
			os.Remove(filepath.Join(dir, "main"))
			assert.NoError(t, err)
		},
	},
	"dotnet": {
		extension:  "cs",
		outputFile: "MyStack.cs",
		check: func(t *testing.T, path string) {
			var err error
			dir := filepath.Dir(path)

			ex, err := executable.FindExecutable("dotnet")
			assert.NoError(t, err, "Failed to find dotnet executable")

			projectFile := filepath.Join(dir, filepath.Base(dir)+".csproj")
			if _, err := ioutil.ReadFile(projectFile); os.IsNotExist(err) {
				defer func() {
					err = os.Remove(projectFile)
					assert.NoError(t, err, "Failed to delete project file")
					err = os.Remove(filepath.Join(dir, "Program.cs"))
					assert.NoError(t, err, "Failed to delete C# project main")
				}()
				err = integration.RunCommand(t, "create dotnet project",
					[]string{ex, "new", "console"}, dir, &integration.ProgramTestOptions{})
				assert.NoError(t, err, "Failed to create C# project")
			}

			// Add dependencies (based on directory name)
			if pkg, pkgVersion := packagesFromNameDotNet(filepath.Base(dir)); pkg != "" {
				err = integration.RunCommand(t, "create dotnet project",
					[]string{ex, "add", "package", pkg, "--version", pkgVersion},
					dir, &integration.ProgramTestOptions{})
				assert.NoError(t, err, "Failed to add dependency %q %q", pkg, pkgVersion)
			} else {
				err = integration.RunCommand(t, "add sdk ref",
					[]string{ex, "add", "reference", "../../../../../../sdk/dotnet/Pulumi"},
					dir, &integration.ProgramTestOptions{})
				assert.NoError(t, err, "Failed to dotnet sdk package reference")
			}

			// Clean up build result
			defer func() {
				err = os.RemoveAll(filepath.Join(dir, "bin"))
				assert.NoError(t, err, "Failed to remove bin result")
				err = os.RemoveAll(filepath.Join(dir, "obj"))
				assert.NoError(t, err, "Failed to remove obj result")
			}()
			err = integration.RunCommand(t, "dotnet build",
				[]string{ex, "build", "--nologo"}, dir, &integration.ProgramTestOptions{})
			assert.NoError(t, err, "Failed to build dotnet project")
		},
	},
}

// TestProgramCodegen runs the complete set of program code generation tests against a particular
// language's code generator.
//
// A program code generation test consists of a PCL file (.pp extension) and a set of expected outputs
// for each language.
//
// The PCL file is the only piece that must be manually authored. Once the schema has been written, the expected outputs
// can be generated by running `PULUMI_ACCEPT=true go test ./..." from the `pkg/codegen` directory.
func TestProgramCodegen(
	t *testing.T,
	language string,
	genProgram func(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error),
) {
	for _, tt := range programTests {
		t.Run(tt.Description, func(t *testing.T) {
			var err error
			if tt.Skip.Has(language) {
				t.Skip()
				return
			}

			expectNYIDiags := tt.ExpectNYIDiags.Has(language)

			var cfg = langConfig[language]

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

			expectedFile := filepath.Join(testDir, tt.Name+"."+cfg.extension)
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
			files, diags, err := genProgram(program)
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
				err := ioutil.WriteFile(expectedFile, files[cfg.outputFile], 0600)
				require.NoError(t, err)
			} else {
				assert.Equal(t, string(expected), string(files[cfg.outputFile]))
			}
			if cfg.check != nil && !tt.SkipCompile.Has(language) {
				cfg.check(t, expectedFile)
			}
		})
	}
}

// packagesFromName attempts to figure out what package should be imported from
// the name of a test.
//
// Example:
// 	"aws-eks-pp" => ("Pulumi.Aws", 4.21.1)
// 	"azure-sa-pp" => ("Pulumi.Azure", 4.21.1)
// 	"resource-options-pp" => ("","")
//
// Note: While we could instead do this by using the generateMetaData function
// for each language, we are trying not to expand the functionality under test.
func packagesFromNameDotNet(name string) (string, string) {
	if strings.Contains(name, "aws") {
		return "Pulumi.Aws", "4.21.1"
	} else if strings.Contains(name, "azure-native") {
		return "Pulumi.AzureNative", "1.29.0"
	} else if strings.Contains(name, "azure") {
		return "Pulumi.Azure", "4.18.0"
	} else if strings.Contains(name, "kubernetes") {
		return "Pulumi.Kubernetes", "3.7.2"
	} else if strings.Contains(name, "random") {
		return "Pulumi.Random", "4.2.0"
	}
	return "", ""
}

// packagesFromNameNodejs attempts to figure out what package should be imported
// from the name of the test.
func packagesFromNameNodejs(name string) (string, string) {
	if strings.Contains(name, "aws") {
		return "@pulumi/aws", "4.21.1"
	} else if strings.Contains(name, "azure-native") {
		return "@pulumi/azure-native", "1.29.0"
	} else if strings.Contains(name, "azure") {
		return "@pulumi/azure", "4.18.0"
	} else if strings.Contains(name, "kubernetes") {
		return "@pulumi/kubernetes", "3.7.2"
	} else if strings.Contains(name, "random") {
		return "@pulumi/random", "4.2.0"
	}
	return "", ""
}
