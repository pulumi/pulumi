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

package pcl

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/stretchr/testify/assert"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return name
}

//nolint:lll
func TestApplyRewriter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input, output string
		skipPromises  bool
	}{
		{
			input:  `"v: ${resource.foo.bar}"`,
			output: `__apply(resource.foo,eval(foo, "v: ${foo.bar}"))`,
		},
		{
			input:  `"v: ${resource.baz[0]}"`,
			output: `__apply(resource.baz,eval(baz, "v: ${baz[0]}"))`,
		},
		{
			input:  `"v: ${resources[0].foo.bar}"`,
			output: `__apply(resources[0].foo,eval(foo, "v: ${foo.bar}"))`,
		},
		{
			input:  `"v: ${resources.*.id[0]}"`,
			output: `__apply(resources.*.id[0],eval(id, "v: ${id}"))`,
		},
		{
			input:  `"v: ${element(resources.*.id, 0)}"`,
			output: `__apply(element(resources.*.id, 0),eval(ids, "v: ${ids}"))`,
		},
		{
			input:  `"v: ${[for r in resources: r.id][0]}"`,
			output: `__apply([for r in resources: r.id][0],eval(id, "v: ${id}"))`,
		},
		{
			input:  `"v: ${element([for r in resources: r.id], 0)}"`,
			output: `__apply(element([for r in resources: r.id], 0),eval(ids, "v: ${ids}"))`,
		},
		{
			input:  `"v: ${resource[key]}"`,
			output: `__apply(resource[key],eval(key, "v: ${key}"))`,
		},
		{
			input:  `"v: ${resource[resource.id]}"`,
			output: `__apply(__apply(resource.id,eval(id, resource[id])),eval(id, "v: ${id}"))`,
		},
		{
			input:  `resourcesPromise.*.id`,
			output: `__apply(resourcesPromise, eval(resourcesPromise, resourcesPromise.*.id))`,
		},
		{
			input:  `[for r in resourcesPromise: r.id]`,
			output: `__apply(resourcesPromise,eval(resourcesPromise, [for r in resourcesPromise: r.id]))`,
		},
		{
			input:  `resourcesOutput.*.id`,
			output: `__apply(resourcesOutput, eval(resourcesOutput, resourcesOutput.*.id))`,
		},
		{
			input:  `[for r in resourcesOutput: r.id]`,
			output: `__apply(resourcesOutput,eval(resourcesOutput, [for r in resourcesOutput: r.id]))`,
		},
		{
			input:  `"v: ${[for r in resourcesPromise: r.id]}"`,
			output: `__apply(__apply(resourcesPromise,eval(resourcesPromise, [for r in resourcesPromise: r.id])),eval(ids, "v: ${ids}"))`,
		},
		{
			input: `toJSON({
										Version = "2012-10-17"
										Statement = [{
											Effect = "Allow"
											Principal = "*"
											Action = [ "s3:GetObject" ]
											Resource = [ "arn:aws:s3:::${resource.id}/*" ]
										}]
									})`,
			output: `__apply(resource.id,eval(id, toJSON({
										Version = "2012-10-17"
										Statement = [{
											Effect = "Allow"
											Principal = "*"
											Action = [ "s3:GetObject" ]
											Resource = [ "arn:aws:s3:::${id}/*" ]
										}]
									})))`,
		},
		{
			input:  `getPromise().property`,
			output: `__apply(getPromise(), eval(getPromise, getPromise.property))`,
		},
		{
			input:  `getPromise().object.foo`,
			output: `__apply(getPromise(), eval(getPromise, getPromise.object.foo))`,
		},
		{
			input:        `getPromise().property`,
			output:       `getPromise().property`,
			skipPromises: true,
		},
		{
			input:        `getPromise().object.foo`,
			output:       `getPromise().object.foo`,
			skipPromises: true,
		},
		{
			input:  `getPromise(resource.id).property`,
			output: `__apply(__apply(resource.id,eval(id, getPromise(id))), eval(getPromise, getPromise.property))`,
		},
		{
			input:  `resource.is ? "yes" : "no"`,
			output: `__apply(resource.is, eval(is, is ? "yes" : "no"))`,
		},
	}

	resourceType := model.NewObjectType(map[string]model.Type{
		"id": model.NewOutputType(model.StringType),
		"foo": model.NewOutputType(model.NewObjectType(map[string]model.Type{
			"bar": model.StringType,
		})),
		"baz": model.NewOutputType(model.NewListType(model.StringType)),
		"is":  model.NewOutputType(model.BoolType),
	})

	scope := model.NewRootScope(syntax.None)
	scope.Define("key", &model.Variable{
		Name:         "key",
		VariableType: model.StringType,
	})
	scope.Define("resource", &model.Variable{
		Name:         "resource",
		VariableType: resourceType,
	})
	scope.Define("resources", &model.Variable{
		Name:         "resources",
		VariableType: model.NewListType(resourceType),
	})
	scope.Define("resourcesPromise", &model.Variable{
		Name:         "resourcesPromise",
		VariableType: model.NewPromiseType(model.NewListType(resourceType)),
	})
	scope.Define("resourcesOutput", &model.Variable{
		Name:         "resourcesOutput",
		VariableType: model.NewOutputType(model.NewListType(resourceType)),
	})
	functions := pulumiBuiltins(bindOptions{})
	scope.DefineFunction("element", functions["element"])
	scope.DefineFunction("toJSON", functions["toJSON"])
	scope.DefineFunction("getPromise", model.NewFunction(model.StaticFunctionSignature{
		Parameters: []model.Parameter{{
			Name: "p",
			Type: model.NewOptionalType(model.StringType),
		}},
		ReturnType: model.NewPromiseType(model.NewObjectType(map[string]model.Type{
			"property": model.StringType,
			"object": model.NewObjectType(map[string]model.Type{
				"foo": model.StringType,
			}),
		})),
	}))

	for _, c := range cases {
		c := c
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()

			expr, diags := model.BindExpressionText(c.input, scope, hcl.Pos{})
			assert.Len(t, diags, 0)

			expr, diags = RewriteApplies(expr, nameInfo(0), !c.skipPromises)
			assert.Len(t, diags, 0)

			assert.Equal(t, c.output, fmt.Sprintf("%v", expr))
		})
	}

	t.Run("skip rewriting applies with toJSON", func(t *testing.T) {
		input := `toJSON({
	Version = "2012-10-17"
	Statement = [{
		Effect = "Allow"
		Principal = "*"
		Action = [ "s3:GetObject" ]
		Resource = [ "arn:aws:s3:::${resource.id}/*" ]
	}]
})`
		expectedOutput := `toJSON({
	Version = "2012-10-17"
	Statement = [{
		Effect = "Allow"
		Principal = "*"
		Action = [ "s3:GetObject" ]
		Resource = [
                __apply(resource.id,eval(id,  "arn:aws:s3:::${id}/*")) ]
	}]
})`

		expr, diags := model.BindExpressionText(input, scope, hcl.Pos{})
		assert.Len(t, diags, 0)

		expr, diags = RewriteAppliesWithSkipToJSON(expr, nameInfo(0), false, true /* skiToJson */)
		assert.Len(t, diags, 0)

		output := fmt.Sprintf("%v", expr)
		assert.Equal(t, expectedOutput, output)
	})
}
