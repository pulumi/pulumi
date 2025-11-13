// Copyright 2020-2024, Pulumi Corporation.
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

package dotnet

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/require"
)

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	test.GenerateDotnetProgramTest(t, GenerateProgram, GenerateProject)
}

func parseAndBindProgram(t *testing.T,
	text string,
	name string,
	testdataPath string,
	options ...pcl.BindOption,
) (*pcl.Program, hcl.Diagnostics, error) {
	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(text), name)
	if err != nil {
		t.Fatalf("could not read %v: %v", name, err)
	}
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	options = append(options, pcl.PluginHost(utils.NewHost(testdataPath)))
	return pcl.BindProgram(parser.Files, options...)
}

func bindProgramWithParameterizedDependencies(t *testing.T) *pcl.Program {
	// usually the base provider here would be `terraform-provider` as this
	// a package descriptor for a dynamically parameterized provider schema.
	// however, for testing purposes we can use `tfe` as it is available
	// in the testdata directory as a parameterized schema.
	source := `
package "tfe" {
  baseProviderName    = "tfe"
  baseProviderVersion = "0.68.2"
  parameterization {
    version = "0.68.2"
    name    = "tfe"
    value   = "eyJyZW1vdGUiOnsidXJsIjoiaGFzaGljb3JwL3RmZSIsInZlcnNpb24iOiIwLjY4LjIifX0="
  }
}


resource "test-organization" "tfe:index/organization:Organization" {
  name  = "my-org-name"
  email = "admin@company.com"
}
`

	program, diags, err := parseAndBindProgram(t,
		source,
		"main.pp",
		filepath.Join("..", "testing", "test", "testdata", "parameterized-schemas"))

	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "unexpected diags: %v", diags)
	require.NotNil(t, program)

	return program
}

// Tests the generated .csproj file when using local dependencies
// that are references to nuget packages
func TestGenerateProjectFileWhenUsingLocalNugetPackages(t *testing.T) {
	t.Parallel()

	program := bindProgramWithParameterizedDependencies(t)
	// local dependencies that uses local nuget packages as used by conformance tests
	localNugetDependencies := map[string]string{
		"tfe": "sdk/tfe/Pulumi.Tfe.0.68.2.nupkg",
	}
	csproj, err := generateProjectFile(program, localNugetDependencies)
	require.NoError(t, err)
	csprojText := string(csproj)
	require.Contains(t, csprojText, `<RestoreSources>sdk/tfe;$(RestoreSources)</RestoreSources>`)
	require.Contains(t, csprojText, `<PackageReference Include="Pulumi.Tfe" Version="0.68.2" />`)
}

// Tests the generated .csproj file when using local dependencies
// that are references to directories containing local SDK
func TestGenerateProjectFileWhenUsingLocalSourcePackages(t *testing.T) {
	t.Parallel()

	program := bindProgramWithParameterizedDependencies(t)
	// local dependencies that uses local nuget packages as used by conformance tests
	localNugetDependencies := map[string]string{
		"tfe": "sdk/tfe",
	}
	csproj, err := generateProjectFile(program, localNugetDependencies)
	require.NoError(t, err)
	csprojText := string(csproj)
	require.Contains(t, csprojText, `<ProjectReference Include="sdk/tfe/Pulumi.Tfe.csproj" />`)
}
