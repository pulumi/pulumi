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

package pcl_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/spf13/afero"

	"github.com/hashicorp/hcl/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestBindProgram(t *testing.T) {
	t.Parallel()

	testdata, err := os.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	bindOptions := map[string][]pcl.BindOption{}
	for _, r := range test.PulumiPulumiProgramTests {
		bindOptions[r.Directory+"-pp"] = r.BindOptions
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, v := range testdata {
		v := v
		if !v.IsDir() {
			continue
		}

		if v.Name() == "self-referencing-components-pp" {
			// skip this test as it is handled separately and is known to error out
			continue
		}

		folderPath := filepath.Join(testdataPath, v.Name())
		files, err := os.ReadDir(folderPath)
		if err != nil {
			t.Fatalf("could not read test data: %v", err)
		}
		for _, fileName := range files {
			fileName := fileName.Name()
			if filepath.Ext(fileName) != ".pp" {
				continue
			}

			t.Run(fileName, func(t *testing.T) {
				t.Parallel()

				path := filepath.Join(folderPath, fileName)
				contents, err := os.ReadFile(path)
				require.NoErrorf(t, err, "could not read %v", path)

				parser := syntax.NewParser()
				err = parser.ParseFile(bytes.NewReader(contents), fileName)
				require.NoErrorf(t, err, "could not read %v", path)
				require.False(t, parser.Diagnostics.HasErrors(), "failed to parse files")

				var bindError error
				var diags hcl.Diagnostics
				loader := pcl.Loader(schema.NewPluginLoader(utils.NewHost(testdataPath)))
				absoluteFolderPath, err := filepath.Abs(folderPath)
				if err != nil {
					t.Fatalf("failed to bind program: unable to find the absolute path of %v", folderPath)
				}
				options := append(
					bindOptions[v.Name()],
					loader,
					pcl.DirPath(absoluteFolderPath),
					pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))
				// PCL binder options are taken from program_driver.go
				program, diags, bindError := pcl.BindProgram(parser.Files, options...)

				require.NoError(t, bindError)
				if diags.HasErrors() || program == nil {
					t.Fatalf("failed to bind program %s: %v", v.Name(), diags)
				}
			})
		}
	}
}

func TestWritingProgramSource(t *testing.T) {
	t.Parallel()
	// STEP 1: Bind the program from {test-data}/components
	componentsDir := "components-pp"
	folderPath := filepath.Join(testdataPath, componentsDir)
	files, err := os.ReadDir(folderPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}
	parser := syntax.NewParser()
	for _, fileName := range files {
		fileName := fileName.Name()
		if filepath.Ext(fileName) != ".pp" {
			continue
		}

		path := filepath.Join(folderPath, fileName)
		contents, err := os.ReadFile(path)
		require.NoErrorf(t, err, "could not read %v", path)

		err = parser.ParseFile(bytes.NewReader(contents), fileName)
		require.NoErrorf(t, err, "could not read %v", path)
		require.False(t, parser.Diagnostics.HasErrors(), "failed to parse files")
	}

	var bindError error
	var diags hcl.Diagnostics
	absoluteProgramPath, err := filepath.Abs(folderPath)
	if err != nil {
		t.Fatalf("failed to bind program: unable to find the absolute path of %v", folderPath)
	}

	program, diags, bindError := pcl.BindProgram(parser.Files,
		pcl.Loader(schema.NewPluginLoader(utils.NewHost(testdataPath))),
		pcl.DirPath(absoluteProgramPath),
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

	require.NoError(t, bindError)
	if diags.HasErrors() || program == nil {
		t.Fatalf("failed to bind program: %v", diags)
	}

	// STEP 2: assert the resulting files
	fs := afero.NewMemMapFs()
	writingFilesError := program.WriteSource(fs)
	require.NoError(t, writingFilesError, "failed to write source files")

	// Assert main file exists
	mainFileExists, err := afero.Exists(fs, "/components.pp")
	require.NoError(t, err, "failed to get the main file")
	assert.True(t, mainFileExists, "main program file should exist at the root")

	// Assert directories "simpleComponent" and "exampleComponent" are present
	simpleComponentDirExists, err := afero.DirExists(fs, "/simpleComponent")
	require.NoError(t, err, "failed to get the simple component dir")
	assert.True(t, simpleComponentDirExists, "simple component dir exists")

	exampleComponentDirExists, err := afero.DirExists(fs, "/exampleComponent")
	require.NoError(t, err, "failed to get the example component dir")
	assert.True(t, exampleComponentDirExists, "example component dir exists")

	// Assert simpleComponent/main.pp and exampleComponent/main.pp exist
	simpleMainExists, err := afero.Exists(fs, "/simpleComponent/main.pp")
	require.NoError(t, err, "failed to get the main file of simple component")
	assert.True(t, simpleMainExists, "main program file of simple component should exist")

	exampleMainExists, err := afero.Exists(fs, "/exampleComponent/main.pp")
	require.NoError(t, err, "failed to get the main file of example component")
	assert.True(t, exampleMainExists, "main program file of example component should exist")
}

func TestConfigNodeTypedString(t *testing.T) {
	t.Parallel()
	source := "config cidrBlock string { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	if err != nil {
		t.Fatalf("could not bind program: %v", err)
	}
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "cidrBlock")
	assert.Equal(t, config.Type(), model.StringType, "the type is a string")
}

func TestConfigNodeTypedOptionalString(t *testing.T) {
	t.Parallel()
	source := "config cidrBlock string { default = null }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "cidrBlock")
	assert.True(t, model.IsOptionalType(config.Type()), "the type is optional")
	elementType := pcl.UnwrapOption(config.Type())
	assert.Equal(t, elementType, model.StringType, "element type is a string")
	assert.True(t, config.Nullable, "The config variable is nullable")
}

func TestConfigNodeTypedInt(t *testing.T) {
	t.Parallel()
	source := "config count int { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "count")
	assert.Equal(t, config.Type(), model.IntType, "the type is a string")
}

func TestConfigNodeTypedStringList(t *testing.T) {
	t.Parallel()
	source := "config names \"list(string)\" { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	listType, ok := config.Type().(*model.ListType)
	assert.True(t, ok, "the type of config is a list type")
	assert.Equal(t, listType.ElementType, model.StringType, "the element type is a string")
}

func TestConfigNodeTypedIntList(t *testing.T) {
	t.Parallel()
	source := "config names \"list(int)\" { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	listType, ok := config.Type().(*model.ListType)
	assert.True(t, ok, "the type of config is a list type")
	assert.Equal(t, listType.ElementType, model.IntType, "the element type is an int")
}

func TestConfigNodeTypedStringMap(t *testing.T) {
	t.Parallel()
	source := "config names \"map(string)\" { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	mapType, ok := config.Type().(*model.MapType)
	assert.True(t, ok, "the type of config is a map type")
	assert.Equal(t, mapType.ElementType, model.StringType, "the element type is a string")
}

func TestConfigNodeTypedIntMap(t *testing.T) {
	t.Parallel()
	source := "config names \"map(int)\" { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	mapType, ok := config.Type().(*model.MapType)
	assert.True(t, ok, "the type of config is a map type")
	assert.Equal(t, mapType.ElementType, model.IntType, "the element type is an int")
}

func TestConfigNodeTypedAnyMap(t *testing.T) {
	t.Parallel()
	source := "config names \"map(any)\" { }"
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	contract.Ignore(diags)
	require.NotNil(t, program, "failed to parse and bind program")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	config, ok := program.Nodes[0].(*pcl.ConfigVariable)
	assert.True(t, ok, "first node is a config variable")
	assert.Equal(t, config.Name(), "names")
	mapType, ok := config.Type().(*model.MapType)
	assert.True(t, ok, "the type of config is a map type")
	assert.Equal(t, mapType.ElementType, model.DynamicType, "the element type is a dynamic")
}

func TestOutputsCanHaveSameNameAsOtherNodes(t *testing.T) {
	t.Parallel()
	// here we have an output with the same name as a config variable
	// this should bind and type-check just fine
	source := `
config cidrBlock string { }
output cidrBlock {
  value = cidrBlock
}
`
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	assert.Equal(t, 0, len(diags), "There are no diagnostics")
	require.NotNil(t, program)
}

func TestUsingDynamicConfigAsRange(t *testing.T) {
	t.Parallel()
	source := `
	config "endpointsServiceNames" {
	  description = "Information about the VPC endpoints to create."
	}

	config "vpcId" "int" {
		description = "The ID of the VPC"
	}

	resource "endpoint" "aws:ec2/vpcEndpoint:VpcEndpoint" {
	  options {
		range = endpointsServiceNames
	  }
	  vpcId             = vpcId
	  serviceName       = range.value.name
	  vpcEndpointType   = range.value.type
	  privateDnsEnabled = range.value.privateDns
	}
`

	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	assert.Equal(t, 0, len(diags), "There are no diagnostics")
	require.NotNil(t, program)
}

func TestLengthFunctionCanBeUsedWithDynamic(t *testing.T) {
	t.Parallel()
	source := `
	config "data" "object({ lambda=object({ subnetIds=list(string) }) })" {
	}
    output "numberOfEndpoints" {
        value = length(data.lambda.subnetIds)
    }
`
	program, diags, err := ParseAndBindProgram(t, source, "config.pp")
	require.NoError(t, err)
	assert.Equal(t, 0, len(diags), "There are no diagnostics")
	require.NotNil(t, program)
}

func TestBindingUnknownResourceWhenSkippingResourceTypeChecking(t *testing.T) {
	t.Parallel()
	source := `
resource provider "pulumi:providers:unknown" { }

resource main "unknown:index:main" {
    first = "hello"
    second = {
        foo = "bar"
    }
}

resource fromModule "unknown:eks:example" {
   options { range = 10 }
   associatedMain = main.id
   anotherValue = main.unknown
}

output "mainId" {
    value = main.id
}

output "values" {
    value = fromModule.values.first
}`

	lenientProgram, lenientDiags, lenientError := ParseAndBindProgram(t, source, "prog.pp", pcl.SkipResourceTypechecking)
	require.NoError(t, lenientError)
	assert.False(t, lenientDiags.HasErrors(), "There are no errors")
	require.NotNil(t, lenientProgram)

	strictProgram, _, strictError := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, strictError, "Binding fails in strict mode")
	assert.Nil(t, strictProgram)
}

func TestBindingUnknownResourceFromKnownSchemaWhenSkippingResourceTypeChecking(t *testing.T) {
	t.Parallel()
	// here the random package is available, but it doesn't have a resource called "Unknown"
	source := `
resource main "random:index:unknown" {
    first = "hello"
    second = {
        foo = "bar"
    }
}

output "mainId" {
    value = main.id
}`

	lenientProgram, lenientDiags, lenientError := ParseAndBindProgram(t, source, "prog.pp", pcl.SkipResourceTypechecking)
	require.NoError(t, lenientError)
	assert.False(t, lenientDiags.HasErrors(), "There are no errors")
	require.NotNil(t, lenientProgram)

	strictProgram, _, strictError := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, strictError, "Binding fails in strict mode")
	assert.Nil(t, strictProgram)
}

func TestBindingUnknownPropertyFromKnownResourceWhenSkippingResourceTypeChecking(t *testing.T) {
	t.Parallel()
	// here the resource declaration is correctly typed but the output `unknownId` references an unknown property
	// this program binds without errors
	source := `
resource randomPet "random:index/randomPet:RandomPet" {
  prefix = "doggo"
}

output "unknownId" {
    value = randomPet.unknownProperty
}

output "knownId" {
    value = randomPet.id
}
`

	lenientProgram, lenientDiags, lenientError := ParseAndBindProgram(t, source, "prog.pp", pcl.SkipResourceTypechecking)
	require.NoError(t, lenientError)
	assert.False(t, lenientDiags.HasErrors(), "There are no errors")
	require.NotNil(t, lenientProgram)

	for _, output := range lenientProgram.OutputVariables() {
		outputType := model.ResolveOutputs(output.Value.Type())
		if output.Name() == "unknownId" {
			assert.Equal(t, model.DynamicType, outputType)
		}

		if output.Name() == "knownId" {
			assert.Equal(t, model.StringType, outputType)
		}
	}

	strictProgram, _, strictError := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, strictError, "Binding fails in strict mode")
	assert.Nil(t, strictProgram)
}

func TestAssigningWrongTypeToResourcePropertyWhenSkippingResourceTypeChecking(t *testing.T) {
	t.Parallel()

	// here the RandomPet resource expects the prefix property to be of type string
	// but we assigned to a boolean. It should still bind when using pcl.SkipResourceTypechecking
	source := `
config data "list(string)" {}
resource randomPet "random:index/randomPet:RandomPet" {
  prefix = data
}`

	lenientProgram, lenientDiags, lenientError := ParseAndBindProgram(t, source, "prog.pp", pcl.SkipResourceTypechecking)
	require.NoError(t, lenientError)
	assert.False(t, lenientDiags.HasErrors(), "There are no errors")
	require.NotNil(t, lenientProgram)

	strictProgram, _, strictError := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, strictError, "Binding fails in strict mode")
	assert.Nil(t, strictProgram)
}

func TestAssigningUnknownPropertyFromKnownResourceWhenSkippingResourceTypeChecking(t *testing.T) {
	t.Parallel()
	// here the resource declaration is assigning an unknown property "unknown" which is not part
	// of the RandomPet inputs.
	source := `
resource randomPet "random:index/randomPet:RandomPet" {
  unknown = "doggo"
}

output "mainId" {
    value = randomPet.unknownProperty
}`

	lenientProgram, lenientDiags, lenientError := ParseAndBindProgram(t, source, "prog.pp", pcl.SkipResourceTypechecking)
	require.NoError(t, lenientError)
	assert.False(t, lenientDiags.HasErrors(), "There are no errors")
	require.NotNil(t, lenientProgram)

	strictProgram, _, strictError := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, strictError, "Binding fails in strict mode")
	assert.Nil(t, strictProgram)
}

func TestTraversalOfOptionalObject(t *testing.T) {
	t.Parallel()
	// foo : Option<{ bar: string }>
	// assert that foo.bar : Option<string>
	source := `
	config "foo" "object({ bar=string })" {
      default = null
      description = "Foo is an optional object because the default is null"
	}

    output "fooBar" {
        value = foo.bar
    }
`

	// first assert that binding the program works
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.NoError(t, err)
	assert.Equal(t, 0, len(diags), "There are no diagnostics")
	require.NotNil(t, program)

	// get the output variable
	outputVars := program.OutputVariables()
	assert.Equal(t, 1, len(outputVars), "There is only one output variable")
	fooBar := outputVars[0]
	fooBarType := fooBar.Value.Type()
	assert.True(t, model.IsOptionalType(fooBarType))
	unwrappedType := pcl.UnwrapOption(fooBarType)
	assert.Equal(t, model.StringType, unwrappedType)
}

func TestBindingInvalidRangeTypeWhenSkippingRangeTypechecking(t *testing.T) {
	t.Parallel()
	// here the range function expects a number but we pass a boolean
	source := `
config "inputRange" "string" { }

data = [for x in inputRange : x]

resource randomPet "random:index/randomPet:RandomPet" {
	options { range = inputRange }
}
`
	// usually a string is not a valid range type
	// but when skipping range typechecking it should still bind
	lenientProgram, lenientDiags, lenientError := ParseAndBindProgram(t, source, "prog.pp", pcl.SkipRangeTypechecking)
	require.NoError(t, lenientError)
	assert.False(t, lenientDiags.HasErrors(), "There are no errors")
	require.NotNil(t, lenientProgram)

	strictProgram, diags, strictError := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, strictError, "Binding fails in strict mode")
	assert.Equal(t, 2, len(diags), "There are two diagnostics")
	assert.Nil(t, strictProgram)
}

func TestTransitivePackageReferencesAreLoadedFromTopLevelResourceDefinition(t *testing.T) {
	t.Parallel()
	// when binding a resource from a package that has a transitive dependency
	// then that transitive dependency is part of the program package references.
	// for example when binding a resource from AWSX package and that resources uses types from the AWS package
	// then both AWSX and AWS packages are part of the program package references
	source := `resource "example" "awsx:ecs:EC2Service" { }`

	program, diags, err := ParseAndBindProgram(t, source, "program.pp", pcl.NonStrictBindOptions()...)
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(), "There are no error diagnostics")
	require.NotNil(t, program)
	assert.Equal(t, 2, len(program.PackageReferences()), "There are two package references")

	packageRefExists := func(pkg string) bool {
		for _, ref := range program.PackageReferences() {
			if ref.Name() == pkg {
				return true
			}
		}

		return false
	}

	assert.True(t, packageRefExists("awsx"), "The program has a reference to the awsx package")
	assert.True(t, packageRefExists("aws"), "The program has a reference to the aws package")
}

func TestAllowMissingVariablesShouldNotErrorOnUnboundVariableReferences(t *testing.T) {
	t.Parallel()
	source := `
resource randomPet "random:index/randomPet:RandomPet" {
	options { parent = parentComponentVariable }
}`

	program, diags, err := ParseAndBindProgram(t, source, "program.pp", pcl.AllowMissingVariables)
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(), "There are no error diagnostics")
	require.NotNil(t, program)
}

func TestBindingComponentFailsWhenReferencingParentAsSource(t *testing.T) {
	t.Parallel()
	source := `component example "." {}`
	program, diags, err := ParseAndBindProgram(t, source, "program.pp",
		pcl.DirPath("."),
		pcl.AllowMissingVariables,
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

	require.Nil(t, program)
	require.NotNil(t, err)
	require.True(t, diags.HasErrors(), "There are error diagnostics")
	require.Contains(t, diags.Error(), "cannot bind component example from the same directory as the parent program")
}

func TestParsingPackageDescriptorsWorks(t *testing.T) {
	t.Parallel()
	source := `
// basic package
package "aws" { }

// package with version
package "azure" { baseProviderVersion = "1.2.3" }

// parameterized package
package "random" {
	baseProviderName = "terraform-provider"
	baseProviderVersion = "0.1.0"
    baseProviderDownloadUrl = "https://example.com/terraform-provider.zip"
    parameterization {
 		name = "random"
        version = "4.5.6"
        value = "SGVsbG8=" // base64 encoded "Hello"
    }
}
`
	parser := syntax.NewParser()
	err := parser.ParseFile(bytes.NewReader([]byte(source)), "program.pp")
	require.NoError(t, err)
	packageDescriptors, diags := pcl.ReadPackageDescriptors(parser.Files[0])
	require.False(t, diags.HasErrors(), "There are no error diagnostics")
	require.Equal(t, 3, len(packageDescriptors), "There are two package descriptors")

	require.Equal(t, "aws", packageDescriptors["aws"].Name)
	require.Nil(t, packageDescriptors["aws"].Version)
	require.Equal(t, "", packageDescriptors["aws"].DownloadURL)
	require.Nil(t, packageDescriptors["aws"].Parameterization)

	require.Equal(t, "azure", packageDescriptors["azure"].Name)
	require.Equal(t, "1.2.3", packageDescriptors["azure"].Version.String())

	assert.Equal(t, "terraform-provider", packageDescriptors["random"].Name)
	assert.Equal(t, "0.1.0", packageDescriptors["random"].Version.String())
	assert.Equal(t, "https://example.com/terraform-provider.zip", packageDescriptors["random"].DownloadURL)
	require.NotNil(t, packageDescriptors["random"].Parameterization)
	assert.Equal(t, "random", packageDescriptors["random"].Parameterization.Name)
	assert.Equal(t, "4.5.6", packageDescriptors["random"].Parameterization.Version.String())
	base64Value := base64.StdEncoding.EncodeToString(packageDescriptors["random"].Parameterization.Value)
	assert.Equal(t, "SGVsbG8=", base64Value)
}

func TestBindingConditionalResourcesDoesNotProduceDiagnostics(t *testing.T) {
	t.Parallel()
	source := `
config "createVpc" "bool" { default = false }
config "subnets" "list(string)" { default = [] }
config "prefixes" "list(string)" { default = [] }

lenPublicSubnets = invoke("std:index:max", {
  input = [
    length(subnets),
    length(prefixes)
  ]
})

resource "defaultVpc" "aws:ec2/vpc:Vpc" {
  options { range = createVpc ? lenPublicSubnets.result : 0 }
  cidrBlock = "10.0.0.1/16"
}
`
	program, diags, err := ParseAndBindProgram(t, source, "program.pp", pcl.NonStrictBindOptions()...)
	require.NoError(t, err)
	assert.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)
}

func TestBindingElementFunctionWithSplatExpression(t *testing.T) {
	t.Parallel()
	source := `
config "randomPrefixes" "list(object({ prefix: string }))" {
	default = []
}

resource "randomPet" "random:index/randomPet:RandomPet" {
	options { range = length(randomPrefixes) }
	prefix = element(randomPrefixes[*].prefix, range.value)
}
`
	// binding in strict mode
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.NoError(t, err)
	assert.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)

	// binding in non-strict mode
	program, diags, err = ParseAndBindProgram(t, source, "program.pp", pcl.NonStrictBindOptions()...)
	require.NoError(t, err)
	assert.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)
}

func TestBindingElementFunctionWithOutputSplatExpression(t *testing.T) {
	t.Parallel()
	source := `
azs = invoke("aws:index:getAvailabilityZones", {})

resource "randomPet" "random:index/randomPet:RandomPet" {
	options { range = length(azs.filters) }
	prefix = element(azs.filters[*].name, range.value)
}
`
	program, diags, err := ParseAndBindProgram(t, source, "program.pp", pcl.PreferOutputVersionedInvokes)
	require.NoError(t, err)
	assert.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)
}

// Tests that valid applications of the `call` intrinsic with simple receivers bind with no errors or diagnostics.
func TestCallValidSimpleReceiver(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
resource "c" "component:index:Callable" {
	value = "bar"
}

cRes1 = call(c, "identity", {})
cRes2 = call(c, "prefixed", { prefix = "foo-" })
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.NoError(t, err)
	require.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)
}

// Tests that valid applications of the `call` intrinsic with complex receivers (e.g. compound expressions that
// eventually result in resources) bind with no errors or diagnostics.
func TestCallValidComplexReceiver(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
resource "c" "component:index:Callable" {
	value = "bar"
}

x = { c = c }

cRes1 = call(x.c, "identity", {})
cRes2 = call(x.c, "prefixed", { prefix = "foo-" })

resource "cs" "component:index:Callable" {
	options { range = 2 }
	value = "quux"
}

csRes1 = call(cs[0], "identity", {})
csRes2 = call(cs[1], "prefixed", { prefix = "baz-" })
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.NoError(t, err)
	require.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)
}

// Tests that applications of the `call` intrinsic with no arguments fail to bind with appropriate diagnostics.
func TestCallNoArguments(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
resource "c" "component:index:Callable" {
	value = "bar"
}

cRes1 = call()
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.ErrorContains(t, err, "call must be passed a receiver, method name, and arguments")
	require.Len(t, diags, 4)
	require.Contains(t, diags[0].Summary, "call must be passed a receiver")
	require.Contains(t, diags[1].Summary, "missing required parameter 'self'")
	require.Contains(t, diags[2].Summary, "missing required parameter 'method'")
	require.Contains(t, diags[3].Summary, "missing required parameter 'args'")
	require.Nil(t, program)
}

// Tests that applications of the `call` intrinsic with a receiver but no method name or arguments fail to bind with
// appropriate diagnostics.
func TestCallNoMethodNameOrMethodArguments(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
resource "c" "component:index:Callable" {
	value = "bar"
}

cRes1 = call(c)
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.ErrorContains(t, err, "call must be passed a receiver, method name, and arguments")
	require.Len(t, diags, 3)
	require.Contains(t, diags[0].Summary, "call must be passed a receiver")
	require.Contains(t, diags[1].Summary, "missing required parameter 'method'")
	require.Contains(t, diags[2].Summary, "missing required parameter 'args'")
	require.Nil(t, program)
}

// Tests that applications of the `call` intrinsic with a receiver and method name but no arguments fail to bind with
// appropriate diagnostics.
func TestCallNoMethodArguments(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
resource "c" "component:index:Callable" {
	value = "bar"
}

cRes1 = call(c, "identity")
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.ErrorContains(t, err, "call must be passed a receiver, method name, and arguments")
	require.Len(t, diags, 2)
	require.Contains(t, diags[0].Summary, "call must be passed a receiver")
	require.Contains(t, diags[1].Summary, "missing required parameter 'args'")
	require.Nil(t, program)
}

// Tests that applications of the `call` intrinsic with an invalid receiver fail to bind with appropriate diagnostics.
func TestCallInvalidReceiver(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
resource "cs" "component:index:Callable" {
	options { range = 2 }
	value = "bar"
}

cRes1 = call(cs[*], "identity", {})
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.ErrorContains(t, err, "call's receiver must be a single resource")
	require.Len(t, diags, 1)
	require.Contains(t, diags[0].Summary, "call's receiver must be a single resource")
	require.Nil(t, program)
}

// Tests that applications of the `call` intrinsic with an invalid method name fail to bind with appropriate
// diagnostics.
func TestCallInvalidMethod(t *testing.T) {
	t.Parallel()

	// Arrange.
	source := `
m = "identity"

resource "c" "component:index:Callable" {
	value = "bar"
}

cRes1 = call(c, "${m}", {})
`

	// Act.
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")

	// Assert.
	require.ErrorContains(t, err, "call's method name must be a string literal")
	require.Len(t, diags, 1)
	require.Contains(t, diags[0].Summary, "call's method name must be a string literal")
	require.Nil(t, program)
}

func TestBindingElementFunctionWithDynamicInput(t *testing.T) {
	t.Parallel()
	source := `
config "data" "any" {}
value = element(data, 0)
`
	// strict mode produces an error: the first argument to 'element' must be a list or tuple
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.NotNil(t, err)
	require.True(t, diags.HasErrors(), "There are error diagnostics")
	require.Contains(t, diags.Error(), "the first argument to 'element' must be a list or tuple")
	require.Nil(t, program)

	// non-strict mode should bind without errors
	program, diags, err = ParseAndBindProgram(t, source, "program.pp", pcl.NonStrictBindOptions()...)
	require.NoError(t, err)
	assert.Empty(t, diags, "There are no error or warning diagnostics")
	require.NotNil(t, program)
}

func TestBindingSelfReferencingResourceFailWithCircularReferenceError(t *testing.T) {
	t.Parallel()
	source := `
resource "randomPet" "random:index/randomPet:RandomPet" {
	prefix = randomPet.prefix
}`
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.Nil(t, program)
	require.NotNil(t, err)
	require.True(t, diags.HasErrors(), "There are error diagnostics")
	require.Contains(t, diags.Error(), "circular reference")
}

func TestBindingMutuallyDependantResourcesFailsWithCircularReferenceError(t *testing.T) {
	t.Parallel()
	source := `
resource "randomPetA" "random:index/randomPet:RandomPet" {
	prefix = randomPetB.prefix
}

resource "randomPetB" "random:index/randomPet:RandomPet" {
	prefix = randomPetA.prefix
}`

	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	require.Nil(t, program)
	require.NotNil(t, err)
	require.True(t, diags.HasErrors(), "There are error diagnostics")
	require.Contains(t, diags.Error(), "circular reference")
}

// Binding a component block that references itself in the same block should fail with a circular reference error
func TestBindingSelfReferencingComponentFailsWithCircularReferenceError(t *testing.T) {
	t.Parallel()
	componentDir := filepath.Join(testdataPath, "self-referencing-components-pp")
	files, err := os.ReadDir(componentDir)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}
	parser := syntax.NewParser()
	for _, fileName := range files {
		fileName := fileName.Name()
		if filepath.Ext(fileName) != ".pp" {
			continue
		}

		path := filepath.Join(componentDir, fileName)
		contents, err := os.ReadFile(path)
		require.NoErrorf(t, err, "could not read %v", path)

		err = parser.ParseFile(bytes.NewReader(contents), fileName)
		require.NoErrorf(t, err, "could not read %v", path)
		require.False(t, parser.Diagnostics.HasErrors(), "failed to parse files")
	}

	var bindError error
	var diags hcl.Diagnostics
	absoluteProgramPath, err := filepath.Abs(componentDir)
	if err != nil {
		t.Fatalf("failed to bind program: unable to find the absolute path of %v", componentDir)
	}

	program, diags, bindError := pcl.BindProgram(parser.Files,
		pcl.Loader(schema.NewPluginLoader(utils.NewHost(testdataPath))),
		pcl.DirPath(absoluteProgramPath),
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

	assert.Nil(t, program)
	require.NotNil(t, bindError)
	assert.True(t, diags.HasErrors(), "There are error diagnostics")
	assert.Contains(t, diags.Error(), "circular reference")
}

func TestBindingMutuallyDependantComponentsSucceeds(t *testing.T) {
	t.Parallel()
	componentDir := filepath.Join(testdataPath, "mutually-dependant-components-pp")
	files, err := os.ReadDir(componentDir)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}
	parser := syntax.NewParser()
	for _, fileName := range files {
		fileName := fileName.Name()
		if filepath.Ext(fileName) != ".pp" {
			continue
		}

		path := filepath.Join(componentDir, fileName)
		contents, err := os.ReadFile(path)
		require.NoErrorf(t, err, "could not read %v", path)

		err = parser.ParseFile(bytes.NewReader(contents), fileName)
		require.NoErrorf(t, err, "could not read %v", path)
		require.False(t, parser.Diagnostics.HasErrors(), "failed to parse files")
	}

	var bindError error
	var diags hcl.Diagnostics
	absoluteProgramPath, err := filepath.Abs(componentDir)
	if err != nil {
		t.Fatalf("failed to bind program: unable to find the absolute path of %v", componentDir)
	}

	program, diags, bindError := pcl.BindProgram(parser.Files,
		pcl.Loader(schema.NewPluginLoader(utils.NewHost(testdataPath))),
		pcl.DirPath(absoluteProgramPath),
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

	require.NotNil(t, program)
	assert.Nil(t, bindError)
	assert.False(t, diags.HasErrors(), "There are no error diagnostics")
}

func TestInferVariableNameForDeferredOutputVariables(t *testing.T) {
	t.Parallel()
	source := "localVariable = component.firstValue"
	program, diags, err := ParseAndBindProgram(t, source, "program.pp", pcl.NonStrictBindOptions()...)
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(), "There are no error or warning diagnostics")
	require.NotNil(t, program)
	var localVariable *pcl.LocalVariable
	for _, v := range program.Nodes {
		switch v := v.(type) {
		case *pcl.LocalVariable:
			localVariable = v
		}
	}

	require.NotNil(t, localVariable, "There is a local variable")
	assert.Equal(t, "localVariable", localVariable.Name())
	traversal, ok := localVariable.Definition.Value.(*model.ScopeTraversalExpression)
	assert.True(t, ok, "The value is a scope traversal expression")
	variableName := pcl.InferVariableName(traversal)
	assert.Equal(t, "componentFirstValue", variableName)
}
