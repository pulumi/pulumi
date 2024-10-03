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

package docs

import (
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func bindTestSchema(t *testing.T, spec schema.PackageSpec) *schema.Package {
	pkg, diags, err := schema.BindSpec(spec, nil)
	assert.Nil(t, diags)
	assert.Nil(t, err)
	return pkg
}

func TestConstructorSyntaxGeneratorForSchema(t *testing.T) {
	t.Parallel()
	pkg := bindTestSchema(t, schema.PackageSpec{
		Name: "test",
		Types: map[string]schema.ComplexTypeSpec{
			"test:index:ExampleEnum": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "string",
				},
				Enum: []schema.EnumValueSpec{
					{Value: "first", Name: "first"},
					{Value: "second", Name: "second"},
				},
			},
			"test:index:ExampleNumericEnum": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "integer",
				},
				Enum: []schema.EnumValueSpec{
					{Value: 10, Name: "first"},
					{Value: 20, Name: "second"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"test:index:First": {
				InputProperties: map[string]schema.PropertySpec{
					"fooString": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
					"fooInt": {
						TypeSpec: schema.TypeSpec{
							Type: "integer",
						},
					},
					"fooBool": {
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
					"fooEnum": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:index:ExampleEnum",
						},
					},
					"fooNumericEnum": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/test:index:ExampleNumericEnum",
						},
					},
				},
			},
			"test:index:Second": {
				InputProperties: map[string]schema.PropertySpec{
					"barString": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
			"test:index:NoInputs": {
				InputProperties: map[string]schema.PropertySpec{},
			},
		},
	})

	languages := []string{"csharp", "go", "nodejs", "python", "yaml", "java"}
	constructorSyntax := generateConstructorSyntaxData(pkg, languages)

	trim := func(s string) string {
		return strings.TrimPrefix(strings.TrimSuffix(s, "\n"), "\n")
	}

	equalPrograms := func(language *languageConstructorSyntax, token string, expected string) {
		program, has := language.resources[token]
		assert.True(t, has, "Expected to find program for token %s", token)
		assert.Equal(t, trim(expected), trim(program))
	}

	expectedResources := 3
	assert.Equal(t, expectedResources, len(constructorSyntax.csharp.resources))
	equalPrograms(constructorSyntax.csharp, "test:index:First", `
var firstResource = new Test.First("firstResource", new()
{
    FooBool = false,
    FooEnum = Test.ExampleEnum.First,
    FooInt = 0,
    FooNumericEnum = Test.ExampleNumericEnum.First,
    FooString = "string",
});
`)

	equalPrograms(constructorSyntax.csharp, "test:index:Second", `
var secondResource = new Test.Second("secondResource", new()
{
    BarString = "string",
});
`)

	equalPrograms(constructorSyntax.csharp, "test:index:NoInputs", `
var noInputsResource = new Test.NoInputs("noInputsResource");
`)

	assert.Equal(t, expectedResources, len(constructorSyntax.typescript.resources))
	equalPrograms(constructorSyntax.typescript, "test:index:First", `
const firstResource = new test.First("firstResource", {
    fooBool: false,
    fooEnum: test.ExampleEnum.First,
    fooInt: 0,
    fooNumericEnum: test.ExampleNumericEnum.First,
    fooString: "string",
});`)
	equalPrograms(constructorSyntax.typescript, "test:index:Second", `
const secondResource = new test.Second("secondResource", {barString: "string"});
`)

	equalPrograms(constructorSyntax.typescript, "test:index:NoInputs", `
const noInputsResource = new test.NoInputs("noInputsResource", {});
`)

	assert.Equal(t, expectedResources, len(constructorSyntax.python.resources))
	equalPrograms(constructorSyntax.python, "test:index:First", `
first_resource = test.First("firstResource",
    foo_bool=False,
    foo_enum=test.ExampleEnum.FIRST,
    foo_int=0,
    foo_numeric_enum=test.ExampleNumericEnum.FIRST,
    foo_string="string")`)
	equalPrograms(constructorSyntax.python, "test:index:Second", `
second_resource = test.Second("secondResource", bar_string="string")`)

	equalPrograms(constructorSyntax.python, "test:index:NoInputs", `
no_inputs_resource = test.NoInputs("noInputsResource")
`)

	assert.Equal(t, expectedResources, len(constructorSyntax.golang.resources))
	equalPrograms(constructorSyntax.golang, "test:index:First", `
_, err := test.NewFirst(ctx, "firstResource", &test.FirstArgs{
	FooBool:        pulumi.Bool(false),
	FooEnum:        index.ExampleEnumFirst,
	FooInt:         pulumi.Int(0),
	FooNumericEnum: index.ExampleNumericEnumFirst,
	FooString:      pulumi.String("string"),
})`)

	equalPrograms(constructorSyntax.golang, "test:index:Second", `
_, err = test.NewSecond(ctx, "secondResource", &test.SecondArgs{
	BarString: pulumi.String("string"),
})`)

	equalPrograms(constructorSyntax.golang, "test:index:NoInputs", `
_, err = test.NewNoInputs(ctx, "noInputsResource", nil)
`)

	assert.Equal(t, expectedResources, len(constructorSyntax.yaml.resources))
	equalPrograms(constructorSyntax.yaml, "test:First", `
type: test:First
properties:
    fooBool: false
    fooEnum: first
    fooInt: 0
    fooNumericEnum: 10
    fooString: string
`)

	equalPrograms(constructorSyntax.yaml, "test:Second", `
type: test:Second
properties:
    barString: string
`)

	equalPrograms(constructorSyntax.yaml, "test:NoInputs", `
type: test:NoInputs
properties: {}
`)

	assert.Equal(t, expectedResources, len(constructorSyntax.java.resources))
	equalPrograms(constructorSyntax.java, "test:index:First", `
var firstResource = new First("firstResource", FirstArgs.builder()
    .fooBool(false)
    .fooEnum("first")
    .fooInt(0)
    .fooNumericEnum(10)
    .fooString("string")
    .build());
`)

	equalPrograms(constructorSyntax.java, "test:index:Second", `
var secondResource = new Second("secondResource", SecondArgs.builder()
    .barString("string")
    .build());
`)

	equalPrograms(constructorSyntax.java, "test:index:NoInputs", `
var noInputsResource = new NoInputs("noInputsResource");
`)
}
