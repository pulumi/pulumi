// Copyright 2016-2020, Pulumi Corporation.
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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:lll, goconst
package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/stretchr/testify/assert"
)

const (
	unitTestTool    = "Pulumi Resource Docs Unit Test"
	providerPackage = "prov"
	codeFence       = "```"
)

var simpleProperties = map[string]schema.PropertySpec{
	"stringProp": {
		Description: "A string prop.",
		TypeSpec: schema.TypeSpec{
			Type: "string",
		},
	},
	"boolProp": {
		Description: "A bool prop.",
		TypeSpec: schema.TypeSpec{
			Type: "boolean",
		},
	},
}

// newTestPackageSpec returns a new fake package spec for a Provider used for testing.
func newTestPackageSpec() schema.PackageSpec {
	pythonMapCase := map[string]schema.RawMessage{
		"python": schema.RawMessage(`{"mapCase":false}`),
	}
	return schema.PackageSpec{
		Name:        providerPackage,
		Version:     "0.0.1",
		Description: "A fake provider package used for testing.",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Types: map[string]schema.ComplexTypeSpec{
			// Package-level types.
			"prov:/getPackageResourceOptions:getPackageResourceOptions": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "Options object for the package-level function getPackageResource.",
					Type:        "object",
					Properties:  simpleProperties,
				},
			},

			// Module-level types.
			"prov:module/getModuleResourceOptions:getModuleResourceOptions": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "Options object for the module-level function getModuleResource.",
					Type:        "object",
					Properties:  simpleProperties,
				},
			},
			"prov:module/ResourceOptions:ResourceOptions": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "The resource options object.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"stringProp": {
							Description: "A string prop.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"boolProp": {
							Description: "A bool prop.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
						},
						"recursiveType": {
							Description: "I am a recursive type.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
							},
						},
					},
				},
			},
			"prov:module/ResourceOptions2:ResourceOptions2": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "The resource options object.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"uniqueProp": {
							Description: "This is a property unique to this type.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Type: "number",
							},
						},
					},
				},
			},
		},
		Provider: schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Description: fmt.Sprintf("The provider type for the %s package.", providerPackage),
				Type:        "object",
			},
			InputProperties: map[string]schema.PropertySpec{
				"stringProp": {
					Description: "A stringProp for the provider resource.",
					TypeSpec: schema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"prov:module2/resource2:Resource2": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: `This is a module-level resource called Resource.
{{% examples %}}
## Example Usage

{{% example %}}
### Basic Example

` + codeFence + `typescript
					// Some TypeScript code.
` + codeFence + `
` + codeFence + `python
					# Some Python code.
` + codeFence + `
{{% /example %}}
{{% example %}}
### Custom Sub-Domain Example

` + codeFence + `typescript
					// Some typescript code
` + codeFence + `
` + codeFence + `python
					# Some Python code.
` + codeFence + `
{{% /example %}}
{{% /examples %}}

## Import

The import docs would be here

` + codeFence + `sh
$ pulumi import prov:module/resource:Resource test test
` + codeFence + `
`,
				},
				InputProperties: map[string]schema.PropertySpec{
					"integerProp": {
						Description: "This is integerProp's description.",
						TypeSpec: schema.TypeSpec{
							Type: "integer",
						},
					},
					"stringProp": {
						Description: "This is stringProp's description.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
					"boolProp": {
						Description: "A bool prop.",
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
					"optionsProp": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
						},
					},
					"options2Prop": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions2:ResourceOptions2",
						},
					},
					"recursiveType": {
						Description: "I am a recursive type.",
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
						},
					},
				},
			},
			"prov:module/resource:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: `This is a module-level resource called Resource.
{{% examples %}}
## Example Usage

{{% example %}}
### Basic Example

` + codeFence + `typescript
					// Some TypeScript code.
` + codeFence + `
` + codeFence + `python
					# Some Python code.
` + codeFence + `
{{% /example %}}
{{% example %}}
### Custom Sub-Domain Example

` + codeFence + `typescript
					// Some typescript code
` + codeFence + `
` + codeFence + `python
					# Some Python code.
` + codeFence + `
{{% /example %}}
{{% /examples %}}

## Import

The import docs would be here

` + codeFence + `sh
$ pulumi import prov:module/resource:Resource test test
` + codeFence + `
`,
				},
				InputProperties: map[string]schema.PropertySpec{
					"integerProp": {
						Description: "This is integerProp's description.",
						TypeSpec: schema.TypeSpec{
							Type: "integer",
						},
					},
					"stringProp": {
						Description: "This is stringProp's description.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
					"boolProp": {
						Description: "A bool prop.",
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
					"optionsProp": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
						},
					},
					"options2Prop": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions2:ResourceOptions2",
						},
					},
					"recursiveType": {
						Description: "I am a recursive type.",
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
						},
					},
				},
			},
			"prov:/packageLevelResource:PackageLevelResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "This is a package-level resource.",
				},
				InputProperties: map[string]schema.PropertySpec{
					"prop": {
						Description: "An input property.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			// Package-level Functions.
			"prov:/getPackageResource:getPackageResource": {
				Description: "A package-level function.",
				Inputs: &schema.ObjectTypeSpec{
					Description: "Inputs for getPackageResource.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"options": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/prov:/getPackageResourceOptions:getPackageResourceOptions",
							},
						},
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Description: "Outputs for getPackageResource.",
					Properties:  simpleProperties,
					Type:        "object",
				},
			},

			// Module-level Functions.
			"prov:module/getModuleResource:getModuleResource": {
				Description: "A module-level function.",
				Inputs: &schema.ObjectTypeSpec{
					Description: "Inputs for getModuleResource.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"options": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/prov:module/getModuleResource:getModuleResource",
							},
						},
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Description: "Outputs for getModuleResource.",
					Properties:  simpleProperties,
					Type:        "object",
				},
			},
		},
	}
}

func getResourceFromModule(resource string, mod *modContext) *schema.Resource {
	for _, r := range mod.resources {
		if resourceName(r) != resource {
			continue
		}
		return r
	}
	return nil
}

func getFunctionFromModule(function string, mod *modContext) *schema.Function {
	for _, f := range mod.functions {
		if tokenToName(f.Token) != function {
			continue
		}
		return f
	}
	return nil
}

func TestFunctionHeaders(t *testing.T) {
	t.Parallel()

	dctx := newDocGenContext()
	testPackageSpec := newTestPackageSpec()

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	tests := []struct {
		ExpectedTitleTag string
		FunctionName     string
		ModuleName       string
		ExpectedMetaDesc string
	}{
		{
			FunctionName: "getPackageResource",
			// Empty string indicates the package-level root module.
			ModuleName:       "",
			ExpectedTitleTag: "prov.getPackageResource",
			ExpectedMetaDesc: "Documentation for the prov.getPackageResource function with examples, input properties, output properties, and supporting types.",
		},
		{
			FunctionName:     "getModuleResource",
			ModuleName:       "module",
			ExpectedTitleTag: "prov.module.getModuleResource",
			ExpectedMetaDesc: "Documentation for the prov.module.getModuleResource function with examples, input properties, output properties, and supporting types.",
		},
	}

	modules := dctx.generateModulesFromSchemaPackage(unitTestTool, schemaPkg)
	for _, test := range tests {
		test := test
		t.Run(test.FunctionName, func(t *testing.T) {
			t.Parallel()

			mod, ok := modules[test.ModuleName]
			if !ok {
				t.Fatalf("could not find the module %s in modules map", test.ModuleName)
			}

			f := getFunctionFromModule(test.FunctionName, mod)
			if f == nil {
				t.Fatalf("could not find %s in modules", test.FunctionName)
			}
			h := mod.genFunctionHeader(f)
			assert.Equal(t, test.ExpectedTitleTag, h.TitleTag)
			assert.Equal(t, test.ExpectedMetaDesc, h.MetaDesc)
		})
	}
}

func TestResourceDocHeader(t *testing.T) {
	t.Parallel()

	dctx := newDocGenContext()
	testPackageSpec := newTestPackageSpec()

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	tests := []struct {
		Name             string
		ExpectedTitleTag string
		ResourceName     string
		ModuleName       string
		ExpectedMetaDesc string
	}{
		{
			Name:         "PackageLevelResourceHeader",
			ResourceName: "PackageLevelResource",
			// Empty string indicates the package-level root module.
			ModuleName:       "",
			ExpectedTitleTag: "prov.PackageLevelResource",
			ExpectedMetaDesc: "Documentation for the prov.PackageLevelResource resource with examples, input properties, output properties, lookup functions, and supporting types.",
		},
		{
			Name:             "ModuleLevelResourceHeader",
			ResourceName:     "Resource",
			ModuleName:       "module",
			ExpectedTitleTag: "prov.module.Resource",
			ExpectedMetaDesc: "Documentation for the prov.module.Resource resource with examples, input properties, output properties, lookup functions, and supporting types.",
		},
	}

	modules := dctx.generateModulesFromSchemaPackage(unitTestTool, schemaPkg)
	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			mod, ok := modules[test.ModuleName]
			if !ok {
				t.Fatalf("could not find the module %s in modules map", test.ModuleName)
			}

			r := getResourceFromModule(test.ResourceName, mod)
			if r == nil {
				t.Fatalf("could not find %s in modules", test.ResourceName)
			}
			h := mod.genResourceHeader(r)
			assert.Equal(t, test.ExpectedTitleTag, h.TitleTag)
			assert.Equal(t, test.ExpectedMetaDesc, h.MetaDesc)
		})
	}
}

func TestExamplesProcessing(t *testing.T) {
	t.Parallel()

	testPackageSpec := newTestPackageSpec()
	dctx := newDocGenContext()

	description := testPackageSpec.Resources["prov:module/resource:Resource"].Description
	docInfo := dctx.decomposeDocstring(description)
	examplesSection := docInfo.examples
	importSection := docInfo.importDetails

	assert.NotEmpty(t, importSection)

	// The resource under test has two examples and both have TS and Python examples.
	assert.Equal(t, 2, len(examplesSection))
	assert.Equal(t, "### Basic Example", examplesSection[0].Title)
	assert.Equal(t, "### Custom Sub-Domain Example", examplesSection[1].Title)
	expectedLangSnippets := []string{"typescript", "python"}
	otherLangSnippets := []string{"csharp", "go"}
	for _, e := range examplesSection {
		for _, lang := range expectedLangSnippets {
			_, ok := e.Snippets[lang]
			assert.True(t, ok, "Could not find %s snippet", lang)
		}
		for _, lang := range otherLangSnippets {
			snippet, ok := e.Snippets[lang]
			assert.True(t, ok, "Expected to find default placeholders for other languages")
			assert.Contains(t, "Coming soon!", snippet)
		}
	}
}

func generatePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	dctx := newDocGenContext()
	dctx.initialize(tool, pkg)
	return dctx.generatePackage(tool, pkg)
}

func TestGeneratePackage(t *testing.T) {
	t.Parallel()

	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "docs",
		GenPackage: generatePackage,
		TestCases:  test.PulumiPulumiSDKTests,
	})
}

func TestDecomposeDocstring(t *testing.T) {
	t.Parallel()
	awsVpcDocs := "Provides a VPC resource.\n" +
		"\n" +
		"{{% examples %}}\n" +
		"## Example Usage\n" +
		"{{% example %}}\n" +
		"\n" +
		"Basic usage:\n" +
		"\n" +
		"```typescript\n" +
		"Basic usage: typescript\n" +
		"```\n" +
		"```python\n" +
		"Basic usage: python\n" +
		"```\n" +
		"```csharp\n" +
		"Basic usage: csharp\n" +
		"```\n" +
		"```go\n" +
		"Basic usage: go\n" +
		"```\n" +
		"```java\n" +
		"Basic usage: java\n" +
		"```\n" +
		"```yaml\n" +
		"Basic usage: yaml\n" +
		"```\n" +
		"\n" +
		"Basic usage with tags:\n" +
		"\n" +
		"```typescript\n" +
		"Basic usage with tags: typescript\n" +
		"```\n" +
		"```python\n" +
		"Basic usage with tags: python\n" +
		"```\n" +
		"```csharp\n" +
		"Basic usage with tags: csharp\n" +
		"```\n" +
		"```go\n" +
		"Basic usage with tags: go\n" +
		"```\n" +
		"```java\n" +
		"Basic usage with tags: java\n" +
		"```\n" +
		"```yaml\n" +
		"Basic usage with tags: yaml\n" +
		"```\n" +
		"\n" +
		"VPC with CIDR from AWS IPAM:\n" +
		"\n" +
		"```typescript\n" +
		"VPC with CIDR from AWS IPAM: typescript\n" +
		"```\n" +
		"```python\n" +
		"VPC with CIDR from AWS IPAM: python\n" +
		"```\n" +
		"```csharp\n" +
		"VPC with CIDR from AWS IPAM: csharp\n" +
		"```\n" +
		"```java\n" +
		"VPC with CIDR from AWS IPAM: java\n" +
		"```\n" +
		"```yaml\n" +
		"VPC with CIDR from AWS IPAM: yaml\n" +
		"```\n" +
		"{{% /example %}}\n" +
		"{{% /examples %}}\n" +
		"\n" +
		"## Import\n" +
		"\n" +
		"VPCs can be imported using the `vpc id`, e.g.,\n" +
		"\n" +
		"```sh\n" +
		" $ pulumi import aws:ec2/vpc:Vpc test_vpc vpc-a01106c2\n" +
		"```\n" +
		"\n" +
		" "
	dctx := newDocGenContext()

	info := dctx.decomposeDocstring(awsVpcDocs)
	assert.Equal(t, docInfo{
		description: "Provides a VPC resource.\n",
		examples: []exampleSection{
			{
				Title: "Basic usage",
				Snippets: map[string]string{
					"csharp":     "```csharp\nBasic usage: csharp\n```\n",
					"go":         "```go\nBasic usage: go\n```\n",
					"java":       "```java\nBasic usage: java\n```\n",
					"python":     "```python\nBasic usage: python\n```\n",
					"typescript": "\n```typescript\nBasic usage: typescript\n```\n",
					"yaml":       "```yaml\nBasic usage: yaml\n```\n",
				},
			},
			{
				Title: "Basic usage with tags",
				Snippets: map[string]string{
					"csharp":     "```csharp\nBasic usage with tags: csharp\n```\n",
					"go":         "```go\nBasic usage with tags: go\n```\n",
					"java":       "```java\nBasic usage with tags: java\n```\n",
					"python":     "```python\nBasic usage with tags: python\n```\n",
					"typescript": "\n```typescript\nBasic usage with tags: typescript\n```\n",
					"yaml":       "```yaml\nBasic usage with tags: yaml\n```\n",
				},
			},
			{
				Title: "VPC with CIDR from AWS IPAM",
				Snippets: map[string]string{
					"csharp":     "```csharp\nVPC with CIDR from AWS IPAM: csharp\n```\n",
					"go":         "Coming soon!",
					"java":       "```java\nVPC with CIDR from AWS IPAM: java\n```\n",
					"python":     "```python\nVPC with CIDR from AWS IPAM: python\n```\n",
					"typescript": "\n```typescript\nVPC with CIDR from AWS IPAM: typescript\n```\n",
					"yaml":       "```yaml\nVPC with CIDR from AWS IPAM: yaml\n```\n",
				},
			},
		},
		importDetails: "\n\nVPCs can be imported using the `vpc id`, e.g.,\n\n```sh\n $ pulumi import aws:ec2/vpc:Vpc test_vpc vpc-a01106c2\n```\n",
	},
		info)
}

func bindSchema(t *testing.T, pkgSpec schema.PackageSpec) *schema.Package {
	pkg, err := schema.ImportSpec(pkgSpec, nil)
	assert.NoError(t, err, "importing spec")
	return pkg
}

func getBoundResource(t *testing.T, pkg *schema.Package, tok string) *schema.Resource {
	for _, r := range pkg.Resources {
		if r.Token == tok {
			return r
		}
	}

	t.Fatalf("could not find resource %s in package", tok)
	return nil
}

func primitiveType(name string) schema.PropertySpec {
	return schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type: name,
		},
	}
}

func testSchemaForCreationExampleSyntax(t *testing.T) *schema.Package {
	pkg := bindSchema(t, schema.PackageSpec{
		Name: "test",
		Resources: map[string]schema.ResourceSpec{
			"test:index:ExampleResource": {
				InputProperties: map[string]schema.PropertySpec{
					"a": primitiveType("string"),
					"b": primitiveType("integer"),
					"c": primitiveType("number"),
					"d": primitiveType("boolean"),
					"e": {
						// Array<string>
						TypeSpec: schema.TypeSpec{
							Type: "array",
							Items: &schema.TypeSpec{
								Type: "string",
							},
						},
					},
					"f": {
						// ExampleObject
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:index:ExampleObject",
						},
					},
					"g": {
						// Array<ExampleObject>
						TypeSpec: schema.TypeSpec{
							Type: "array",
							Items: &schema.TypeSpec{
								Ref: "#/types/test:index:ExampleObject",
							},
						},
					},
					"h": {
						// Map<string>
						TypeSpec: schema.TypeSpec{
							Type: "object",
							AdditionalProperties: &schema.TypeSpec{
								Type: "string",
							},
						},
					},
					"i": {
						// Map<ExampleObject>
						TypeSpec: schema.TypeSpec{
							Type: "object",
							AdditionalProperties: &schema.TypeSpec{
								Ref: "#/types/test:index:ExampleObject",
							},
						},
					},
					"j": {
						// Enum
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:index:ExampleEnum",
						},
					},
					"k": {
						// Array<Enum>
						TypeSpec: schema.TypeSpec{
							Type: "array",
							Items: &schema.TypeSpec{
								Ref: "#/types/test:index:ExampleEnum",
							},
						},
					},
					"l": {
						// Asset
						TypeSpec: schema.TypeSpec{
							Ref: "pulumi.json#/Asset",
						},
					},
					"m": {
						// Archive
						TypeSpec: schema.TypeSpec{
							Ref: "pulumi.json#/Archive",
						},
					},
				},
			},
			"test:s3:Bucket": {
				InputProperties: map[string]schema.PropertySpec{
					"bucketName": primitiveType("string"),
				},
			},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"test:index:ExampleObject": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"x": primitiveType("string"),
						"y": primitiveType("string"),
					},
				},
			},
			"test:index:ExampleEnum": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "string",
				},
				Enum: []schema.EnumValueSpec{
					{Name: "First", Value: "FIRST"},
					{Name: "Second", Value: "SECOND"},
				},
			},
		},
	})

	return pkg
}

func testSchemaWithRecursiveObjectType(t *testing.T) *schema.Package {
	pkg := bindSchema(t, schema.PackageSpec{
		Name: "test",
		Resources: map[string]schema.ResourceSpec{
			"test:index:ExampleResource": {
				InputProperties: map[string]schema.PropertySpec{
					"recursiveObject": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:example:Recursive",
						},
					},
				},
			},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"test:example:Recursive": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"anotherInput": primitiveType("string"),
						"recursiveType": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/test:example:Recursive",
							},
						},
					},
				},
			},
		},
	})

	return pkg
}

func TestCreationExampleSyntaxForYAML(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxYAML(exampleResource)
	expected := `
name: example
runtime: yaml
resources:
  exampleResource:
    type: test:ExampleResource
    properties:
      a: "string"
      b: 0
      c: 0.0
      d: true|false
      e: ["string"]
      f: 
        x: "string"
        y: "string"
      g: [
        x: "string"
        y: "string"
      ]
      h: 
        "string": "string"
      i: 
        "string": 
          x: "string"
          y: "string"

      j: FIRST|SECOND
      k: [FIRST|SECOND]
      l: 
        Fn::StringAsset: "example content"
      m: 
        Fn::FileAsset: ./file.txt
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForYAMLWithModule(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:s3:Bucket")
	creationExample := genCreationExampleSyntaxYAML(exampleResource)
	expected := `
name: example
runtime: yaml
resources:
  bucket:
    type: test:s3:Bucket
    properties:
      bucketName: "string"
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleUsingRecursiveTypeForYAML(t *testing.T) {
	t.Parallel()

	schema := testSchemaWithRecursiveObjectType(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxYAML(exampleResource)
	expected := `
name: example
runtime: yaml
resources:
  exampleResource:
    type: test:ExampleResource
    properties:
      recursiveObject: 
        anotherInput: "string"
        recursiveType: type(test:example:Recursive)
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForTypescript(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxTypescript(exampleResource)
	expected := `
import * as pulumi from "@pulumi/pulumi";
import * as test from "@pulumi/test";

const exampleResource = new test.ExampleResource("exampleResource", {
  a: "string",
  b: 0,
  c: 0.0,
  d: true|false,
  e: ["string"],
  f: {
    x: "string",
    y: "string",
  },
  g: [{
    x: "string",
    y: "string",
  }],
  h: {
    "string": "string"
  },
  i: {
    "string": {
      x: "string",
      y: "string",
    }
  },
  j: "FIRST|SECOND",
  k: ["FIRST|SECOND"],
  l: new pulumi.asset.StringAsset("Hello, world!"),
  m: new pulumi.asset.FileAsset("./file.txt"),
});
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForTypescriptWithModule(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:s3:Bucket")
	creationExample := genCreationExampleSyntaxTypescript(exampleResource)
	expected := `
import * as pulumi from "@pulumi/pulumi";
import * as test from "@pulumi/test";

const bucket = new test.s3.Bucket("bucket", {
  bucketName: "string",
});
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleUsingRecursiveTypeForTypescript(t *testing.T) {
	t.Parallel()

	schema := testSchemaWithRecursiveObjectType(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxTypescript(exampleResource)
	expected := `
import * as pulumi from "@pulumi/pulumi";
import * as test from "@pulumi/test";

const exampleResource = new test.ExampleResource("exampleResource", {
  recursiveObject: {
    anotherInput: "string",
    recursiveType: type(test:example:Recursive),
  },
});
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForCSharp(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxCSharp(exampleResource)
	expected := `
using Pulumi;
using Test = Pulumi.Test;

var exampleResource = new Test.ExampleResource("exampleResource", new () 
{
  A = "string",
  B = 0,
  C = 0.0,
  D = true|false,
  E = new []
  {
    "string"
  },
  F = new Test.Inputs.ExampleObjectArgs
  {
    X = "string",
    Y = "string",
  },
  G = new []
  {
    new Test.Inputs.ExampleObjectArgs
    {
      X = "string",
      Y = "string",
    }
  },
  H = {
    ["string"] = "string"
  },
  I = {
    ["string"] = new Test.Inputs.ExampleObjectArgs
    {
      X = "string",
      Y = "string",
    }
  },
  J = "FIRST|SECOND",
  K = new []
  {
    "FIRST|SECOND"
  },
  L = new StringAsset("Hello, world!"),
  M = new FileAsset("./file.txt"),
});
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForCSharpWithModule(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:s3:Bucket")
	creationExample := genCreationExampleSyntaxCSharp(exampleResource)
	expected := `
using Pulumi;
using Test = Pulumi.Test;

var bucket = new Test.S3.Bucket("bucket", new () 
{
  BucketName = "string",
});
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleUsingRecursiveTypeForCSharp(t *testing.T) {
	t.Parallel()

	schema := testSchemaWithRecursiveObjectType(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxCSharp(exampleResource)
	expected := `
using Pulumi;
using Test = Pulumi.Test;

var exampleResource = new Test.ExampleResource("exampleResource", new () 
{
  RecursiveObject = new Test.Example.Inputs.RecursiveArgs
  {
    AnotherInput = "string",
    RecursiveType = type(test:example:Recursive),
  },
});
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForPython(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxPython(exampleResource)
	expected := `
import pulumi
import pulumi_test as test

exampleResource = test.ExampleResource("exampleResource",
  a="string",
  b=0,
  c=0.0,
  d=True|False,
  e=[
    "string",
  ],
  f=test.ExampleObjectArgs(
    x="string",
    y="string",
  ),
  g=[
    test.ExampleObjectArgs(
      x="string",
      y="string",
    ),
  ],
  h={
    'string': "string"
  },
  i={
    'string': test.ExampleObjectArgs(
      x="string",
      y="string",
    )
  },
  j="FIRST|SECOND",
  k=[
    "FIRST|SECOND",
  ],
  l=pulumi.StringAsset("Hello, world!"),
  m=pulumi.FileAsset("./file.txt")
)
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForPythonWithModule(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:s3:Bucket")
	creationExample := genCreationExampleSyntaxPython(exampleResource)
	expected := `
import pulumi
import pulumi_test as test

bucket = test.s3.Bucket("bucket",
  bucket_name="string"
)
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleUsingRecursiveTypeForPython(t *testing.T) {
	t.Parallel()

	schema := testSchemaWithRecursiveObjectType(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxPython(exampleResource)
	expected := `
import pulumi
import pulumi_test as test

exampleResource = test.ExampleResource("exampleResource",
  recursive_object=test.example.RecursiveArgs(
    another_input="string",
    recursive_type=type(test:example:Recursive),
  )
)
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForJava(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxJava(exampleResource)
	expected := `
import com.pulumi.Pulumi;
import java.util.List;
import java.util.Map;

var exampleResource = new ExampleResource("exampleResource", ExampleResourceArgs.builder()
  .a("string")
  .b(0)
  .c(0.0)
  .d(true|false)
  .e(List.of("string"))
  .f(ExampleObjectArgs.builder()
    .x("string")
    .y("string")
    .build())
  .g(List.of(
    ExampleObjectArgs.builder()
      .x("string")
      .y("string")
      .build()
  ))
  .h(Map.ofEntries(
    Map.entry("string", "string")
  ))
  .i(Map.ofEntries(
    Map.entry("string", ExampleObjectArgs.builder()
      .x("string")
      .y("string")
      .build())
  ))
  .j("FIRST|SECOND")
  .k(List.of("FIRST|SECOND"))
  .l(new StringAsset("Hello, world!"))
  .m(new FileAsset("./file.txt"))
  .build());
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForJavaWithModule(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:s3:Bucket")
	creationExample := genCreationExampleSyntaxJava(exampleResource)
	expected := `
import com.pulumi.Pulumi;
import java.util.List;
import java.util.Map;

var bucket = new Bucket("bucket", BucketArgs.builder()
  .bucketName("string")
  .build());
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleUsingRecursiveTypeForJava(t *testing.T) {
	t.Parallel()

	schema := testSchemaWithRecursiveObjectType(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxJava(exampleResource)
	expected := `
import com.pulumi.Pulumi;
import java.util.List;
import java.util.Map;

var exampleResource = new ExampleResource("exampleResource", ExampleResourceArgs.builder()
  .recursiveObject(RecursiveArgs.builder()
    .anotherInput("string")
    .recursiveType(type(test:example:Recursive))
    .build())
  .build());
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForGo(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxGo(exampleResource)
	expected := `
import (
  "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
  "github.com/pulumi/pulumi-test/sdk/v3/go/test"
)

exampleResource, err := test.NewExampleResource("exampleResource", &test.ExampleResourceArgs{
  A: pulumi.String("string"),
  B: pulumi.Int(0),
  C: pulumi.Float64(0.0),
  D: pulumi.Bool(true|false),
  E: pulumi.StringArray{
    pulumi.String("string")
  },
  F: &test.ExampleObjectArgs{
    X: pulumi.String("string"),
    Y: pulumi.String("string"),
  },
  G: test.ExampleObjectArray{
    &test.ExampleObjectArgs{
      X: pulumi.String("string"),
      Y: pulumi.String("string"),
    }
  },
  H: pulumi.StringMap{
    "string": pulumi.String("string")
  },
  I: test.ExampleObjectMap{
    "string": &test.ExampleObjectArgs{
      X: pulumi.String("string"),
      Y: pulumi.String("string"),
    }
  },
  J: ExampleEnumFirst|ExampleEnumSecond,
  K: test.ExampleEnumArray{
    ExampleEnumFirst|ExampleEnumSecond
  },
  L: pulumi.NewStringAsset("Hello, world!"),
  M: pulumi.NewFileArchive("./file.txt"),
})
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleSyntaxForGoWithModule(t *testing.T) {
	t.Parallel()

	schema := testSchemaForCreationExampleSyntax(t)
	exampleResource := getBoundResource(t, schema, "test:s3:Bucket")
	creationExample := genCreationExampleSyntaxGo(exampleResource)
	expected := `
import (
  "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
  "github.com/pulumi/pulumi-test/sdk/v3/go/test/s3"
)

bucket, err := s3.NewBucket("bucket", &s3.BucketArgs{
  BucketName: pulumi.String("string"),
})
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func TestCreationExampleUsingRecursiveTypeForGo(t *testing.T) {
	t.Parallel()

	schema := testSchemaWithRecursiveObjectType(t)
	exampleResource := getBoundResource(t, schema, "test:index:ExampleResource")
	creationExample := genCreationExampleSyntaxGo(exampleResource)
	expected := `
import (
  "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
  "github.com/pulumi/pulumi-test/sdk/v3/go/test"
)

exampleResource, err := test.NewExampleResource("exampleResource", &test.ExampleResourceArgs{
  RecursiveObject: &example.RecursiveArgs{
    AnotherInput: pulumi.String("string"),
    RecursiveType: type(test:example:Recursive),
  },
})
`
	assert.Equal(t, strings.TrimPrefix(expected, "\n"), creationExample)
}

func readSchemaFile(file string) (pkgSpec schema.PackageSpec) {
	// Read in, decode, and import the schema.
	schemaBytes, err := os.ReadFile(filepath.Join("..", "testing", "test", "testdata", file))
	if err != nil {
		panic(err)
	}

	if strings.HasSuffix(file, ".json") {
		if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
			panic(err)
		}
	} else if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
		if err = yaml.Unmarshal(schemaBytes, &pkgSpec); err != nil {
			panic(err)
		}
	} else {
		panic("unknown schema file extension while parsing " + file)
	}

	return pkgSpec
}

func TestCreationExampleForLambdaFunction(t *testing.T) {
	t.Parallel()

	pkgSpec := readSchemaFile("aws-5.16.2.json")
	pkg := bindSchema(t, pkgSpec)
	lambdaFunction := getBoundResource(t, pkg, "aws:lambda/function:Function")
	exampleTypescript := genCreationExampleSyntaxTypescript(lambdaFunction)
	examplePython := genCreationExampleSyntaxPython(lambdaFunction)
	exampleCSharp := genCreationExampleSyntaxCSharp(lambdaFunction)
	exampleGo := genCreationExampleSyntaxGo(lambdaFunction)
	exampleJava := genCreationExampleSyntaxJava(lambdaFunction)
	exampleYaml := genCreationExampleSyntaxYAML(lambdaFunction)

	fullDocument := "### TypeScript\n\n" + "```typescript\n" + exampleTypescript + "```\n\n" +
		"### Python\n\n" + "```python\n" + examplePython + "```\n\n" +
		"### C#\n\n" + "```csharp\n" + exampleCSharp + "```\n\n" +
		"### Go\n\n" + "```go\n" + exampleGo + "```\n\n" +
		"### Java\n\n" + "```java\n" + exampleJava + "```\n\n" +
		"### YAML\n\n" + "```yaml\n" + exampleYaml + "```\n\n"

	expectedFileContent, err := os.ReadFile(filepath.Join("example_creation_testdata", "aws-lambda-function.md"))
	assert.NoError(t, err)
	assert.Equal(t, string(expectedFileContent), fullDocument)
}

func TestCreationExampleForKubernetesAppsDeployment(t *testing.T) {
	t.Parallel()

	pkgSpec := readSchemaFile("kubernetes-3.7.2.json")
	pkg := bindSchema(t, pkgSpec)
	deployment := getBoundResource(t, pkg, "kubernetes:apps/v1:Deployment")
	exampleTypescript := genCreationExampleSyntaxTypescript(deployment)
	examplePython := genCreationExampleSyntaxPython(deployment)
	exampleCSharp := genCreationExampleSyntaxCSharp(deployment)
	exampleGo := genCreationExampleSyntaxGo(deployment)
	exampleJava := genCreationExampleSyntaxJava(deployment)
	exampleYaml := genCreationExampleSyntaxYAML(deployment)

	fullDocument := "### TypeScript\n\n" + "```typescript\n" + exampleTypescript + "```\n\n" +
		"### Python\n\n" + "```python\n" + examplePython + "```\n\n" +
		"### C#\n\n" + "```csharp\n" + exampleCSharp + "```\n\n" +
		"### Go\n\n" + "```go\n" + exampleGo + "```\n\n" +
		"### Java\n\n" + "```java\n" + exampleJava + "```\n\n" +
		"### YAML\n\n" + "```yaml\n" + exampleYaml + "```\n\n"

	expectedFileContent, err := os.ReadFile(filepath.Join("example_creation_testdata", "k8s-deployment.md"))
	assert.NoError(t, err)
	assert.Equal(t, string(expectedFileContent), fullDocument)
}
