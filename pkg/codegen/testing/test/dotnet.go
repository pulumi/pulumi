// Copyright 2022-2024, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func GenerateDotnetProgramTest(
	t *testing.T,
	genProgram GenProgram,
	genProject GenProject,
) {
	expectedVersion := map[string]PkgVersionInfo{
		"aws-resource-options-4.26": {
			Pkg:          "<PackageReference Include=\"Pulumi.Aws\"",
			OpAndVersion: "Version=\"4.26.0\"",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "<PackageReference Include=\"Pulumi.Aws\"",
			OpAndVersion: "Version=\"5.16.2\"",
		},
	}

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				checkDotnet(t, path, dependencies, "")
			},
			GenProgram: genProgram,
			TestCases: []ProgramTest{
				{
					Directory:   "aws-resource-options-4.26",
					Description: "Resource Options",
				},
				{
					Directory:   "aws-resource-options-5.16.2",
					Description: "Resource Options",
				},
			},

			IsGenProject:    true,
			GenProject:      genProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "test.csproj",
		},
	)
}

func GenerateDotnetBatchTest(t *testing.T, rootDir string, genProgram GenProgram, testCases []ProgramTest) {
	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				checkDotnet(t, path, dependencies, "")
			},
			GenProgram: genProgram,
			TestCases:  testCases,
		},
	)
}

func GenerateDotnetYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	err := os.Chdir(filepath.Join(rootDir, "pkg", "codegen", "dotnet"))
	assert.NoError(t, err)

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				checkDotnet(t, path, dependencies, "")
			},
			GenProgram: genProgram,
			TestCases:  PulumiPulumiYAMLProgramTests,
		},
	)
}

func checkDotnet(t *testing.T, path string, dependencies codegen.StringSet, pulumiSDKPath string) {
	var err error
	dir := filepath.Dir(path)

	ex, err := executable.FindExecutable("dotnet")
	require.NoError(t, err, "Failed to find dotnet executable")

	// We create a new cs-project each time the test is run.
	projectFile := filepath.Join(dir, filepath.Base(dir)+".csproj")
	programFile := filepath.Join(dir, "Program.cs")
	if err = os.Remove(projectFile); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	if err = os.Remove(programFile); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	err = integration.RunCommand(t, "create dotnet project",
		[]string{ex, "new", "console", "-f", "net8.0"}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to create C# project")

	// Remove Program.cs again generated from "dotnet new console"
	// because the generated C# program already has an entry point
	if err = os.Remove(programFile); !os.IsNotExist(err) {
		require.NoError(t, err)
	}

	// Add dependencies
	pkgs := dotnetDependencies(dependencies)
	if len(pkgs) != 0 {
		for _, pkg := range pkgs {
			pkg.install(t, ex, dir)
		}
		dep{"Pulumi", PulumiDotnetSDKVersion}.install(t, ex, dir)
	} else {
		// We would like this regardless of other dependencies, but dotnet
		// packages do not play well with package references.
		if pulumiSDKPath != "" {
			err = integration.RunCommand(t, "add sdk ref",
				[]string{ex, "add", "reference", pulumiSDKPath},
				dir, &integration.ProgramTestOptions{})
			require.NoError(t, err, "Failed to dotnet sdk package reference")
		} else {
			dep{"Pulumi", PulumiDotnetSDKVersion}.install(t, ex, dir)
		}
	}

	// Clean up build result
	defer func() {
		err = os.RemoveAll(filepath.Join(dir, "bin"))
		assert.NoError(t, err, "Failed to remove bin result")
		err = os.RemoveAll(filepath.Join(dir, "obj"))
		assert.NoError(t, err, "Failed to remove obj result")
	}()
	typeCheckDotnet(t, path, dependencies, pulumiSDKPath)
}

func typeCheckDotnet(t *testing.T, path string, dependencies codegen.StringSet, pulumiSDKPath string) {
	var err error
	dir := filepath.Dir(path)

	ex, err := executable.FindExecutable("dotnet")
	require.NoError(t, err)

	err = integration.RunCommand(t, "dotnet build",
		[]string{ex, "build", "--nologo"}, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to build dotnet project")
}

type dep struct {
	Name    string
	Version string
}

func (pkg dep) install(t *testing.T, ex, dir string) {
	args := []string{ex, "add", "package", pkg.Name}
	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}
	err := integration.RunCommand(t, "Add package",
		args, dir, &integration.ProgramTestOptions{})
	require.NoError(t, err, "Failed to add dependency %q %q", pkg.Name, pkg.Version)
}

// Converts from the hcl2 dependency format to the dotnet format.
//
// Example:
//
//	"aws" => {"Pulumi.Aws", 4.21.1}
//	"azure" => {"Pulumi.Azure", 4.21.1}
func dotnetDependencies(deps codegen.StringSet) []dep {
	result := make([]dep, len(deps))
	for i, d := range deps.SortedValues() {
		switch d {
		case "aws":
			result[i] = dep{"Pulumi.Aws", AwsSchema}
		case "azure-native":
			result[i] = dep{"Pulumi.AzureNative", AzureNativeSchema}
		case "azure":
			// TODO: update constant in test.AzureSchema to v5.x
			// because it has output-versioned function invokes
			result[i] = dep{"Pulumi.Azure", "5.12.0"}
		case "kubernetes":
			result[i] = dep{"Pulumi.Kubernetes", KubernetesSchema}
		case "random":
			result[i] = dep{"Pulumi.Random", RandomSchema}
		case "aws-static-website":
			result[i] = dep{"Pulumi.AwsStaticWebsite", AwsStaticWebsiteSchema}
		case "aws-native":
			result[i] = dep{"Pulumi.AwsNative", AwsNativeSchema}
		default:
			runes := []rune(d)
			titlecase := append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...)
			result[i] = dep{"Pulumi." + string(titlecase), ""}
		}
	}
	return result
}
