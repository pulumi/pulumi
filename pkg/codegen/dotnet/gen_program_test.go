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
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
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

	// Prepend the default host so that we can override if necessary.
	options = append([]pcl.BindOption{pcl.PluginHost(utils.NewHost(testdataPath))}, options...)
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

func bindProgramWithReplacementTrigger(t *testing.T) *pcl.Program {
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
  options {
    replacementTrigger = secret("test-secret")
  }
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

func TestGenerateProgramWithReplacementTrigger(t *testing.T) {
	t.Parallel()

	program := bindProgramWithReplacementTrigger(t)
	files, diags, err := GenerateProgram(program)
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "unexpected diags: %v", diags)
	require.NotNil(t, files)

	programFile, ok := files["Program.cs"]
	require.True(t, ok, "Program.cs should be generated")

	programText := string(programFile)

	require.Contains(t, programText, "ReplacementTrigger", "generated code should contain ReplacementTrigger")
	require.Contains(t, programText, "Output.CreateSecret", "generated code should contain secret replacement trigger")
}

// We have to invent an inline loader to produce an import that will cause a namespace collision.
// In this case, that collision is between `Pulumi.Output` and `Output` in `Pulumi`.
func bindProgramWithNamespaceCollision(t *testing.T) *pcl.Program {
	source := `
resource "test-resource" "output:index:Resource" {
    value = 1
}
`

	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(source), "main.pp")
	require.NoError(t, err)
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	loader := &inlineLoader{
		schemas: map[string]schema.PackageSpec{
			"output": {
				Name:    "output",
				Version: "1.0.0",
				Resources: map[string]schema.ResourceSpec{
					"output:index:Resource": {
						ObjectTypeSpec: schema.ObjectTypeSpec{
							Type: "object",
							Properties: map[string]schema.PropertySpec{
								"value": {TypeSpec: schema.TypeSpec{Type: "number"}},
							},
							Required: []string{"value"},
						},
						InputProperties: map[string]schema.PropertySpec{
							"value": {TypeSpec: schema.TypeSpec{Type: "number"}},
						},
						RequiredInputs: []string{"value"},
					},
				},
			},
		},
	}

	program, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(loader))
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "unexpected diags: %v", diags)
	require.NotNil(t, program)

	return program
}

// Tests that namespace collision code is correctly exercised.
func TestGenerateProgramWithNamespaceCollision(t *testing.T) {
	t.Parallel()

	program := bindProgramWithNamespaceCollision(t)
	files, diags, err := GenerateProgram(program)
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "unexpected diags: %v", diags)
	require.NotNil(t, files)

	programFile, ok := files["Program.cs"]
	require.True(t, ok, "Program.cs should be generated")

	programText := string(programFile)
	t.Logf("Generated Program.cs:\n%s", programText)

	require.Contains(t, programText, `using PulumiOutput = Pulumi.Output;`,
		"generated code should contain PulumiOutput alias")
	require.Contains(t, programText, `new PulumiOutput.Resource`, "generated code should use PulumiOutput alias")
}

func bindProgramWithFunctionNamespaceCollision(t *testing.T) *pcl.Program {
	source := `
result = invoke("output:index:Output", {
    value = "test"
})
`

	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(source), "main.pp")
	require.NoError(t, err)
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	loader := &inlineLoader{
		schemas: map[string]schema.PackageSpec{
			"output": {
				Name:    "output",
				Version: "1.0.0",
				Functions: map[string]schema.FunctionSpec{
					"output:index:Output": {
						Description: "A function named Output that causes namespace collision",
						Inputs: &schema.ObjectTypeSpec{
							Type: "object",
							Properties: map[string]schema.PropertySpec{
								"value": {TypeSpec: schema.TypeSpec{Type: "string"}},
							},
							Required: []string{"value"},
						},
						ReturnType: &schema.ReturnTypeSpec{
							TypeSpec: &schema.TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	program, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(loader))
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "unexpected diags: %v", diags)
	require.NotNil(t, program)

	return program
}

func TestGenerateProgramWithFunctionNamespaceCollision(t *testing.T) {
	t.Parallel()

	program := bindProgramWithFunctionNamespaceCollision(t)
	files, diags, err := GenerateProgram(program)
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "unexpected diags: %v", diags)
	require.NotNil(t, files)

	programFile, ok := files["Program.cs"]
	require.True(t, ok, "Program.cs should be generated")

	programText := string(programFile)
	t.Logf("Generated Program.cs:\n%s", programText)

	require.Contains(t, programText, `using PulumiOutput = Pulumi.Output;`,
		"generated code should contain PulumiOutput alias for Pulumi.Output namespace")
	require.Contains(t, programText, `PulumiOutput.Output.Invoke`,
		"generated code should use PulumiOutput alias for function")
}

type inlineLoader struct {
	schemas map[string]schema.PackageSpec
}

func (l *inlineLoader) LoadPackageReference(pkg string, version *semver.Version) (schema.PackageReference, error) {
	return l.LoadPackageReferenceV2(context.TODO(), &schema.PackageDescriptor{Name: pkg, Version: version})
}

func (l *inlineLoader) LoadPackageReferenceV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (schema.PackageReference, error) {
	if descriptor.Name == "pulumi" {
		return schema.DefaultPulumiPackage.Reference(), nil
	}
	spec, ok := l.schemas[descriptor.Name]
	if !ok {
		return nil, nil
	}
	pkg, diags, err := schema.BindSpec(spec, l, schema.ValidationOptions{AllowDanglingReferences: true})
	if err != nil || diags.HasErrors() {
		return nil, err
	}
	return pkg.Reference(), nil
}

func (l *inlineLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil || ref == nil {
		return nil, err
	}
	return ref.Definition()
}

func (l *inlineLoader) LoadPackageV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (*schema.Package, error) {
	ref, err := l.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil || ref == nil {
		return nil, err
	}
	return ref.Definition()
}
