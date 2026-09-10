// Copyright 2026, Pulumi Corporation.
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

package nodejs

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProxyApplyWithCallExpression(t *testing.T) {
	t.Parallel()

	resultType := model.NewObjectType(map[string]model.Type{
		"result": model.NewOptionalType(model.StringType),
	})
	call := &model.FunctionCallExpression{
		Name: "call",
		Signature: model.StaticFunctionSignature{
			ReturnType: model.NewOutputType(resultType),
		},
	}
	parameter := &model.Variable{Name: "result", VariableType: resultType}
	body := &model.RelativeTraversalExpression{
		Source: model.VariableReference(parameter),
		Traversal: hcl.Traversal{
			hcl.TraverseAttr{Name: "result"},
		},
	}
	require.Empty(t, body.Typecheck(false))

	parameters := mapset.NewSet[model.Traversable]()
	parameters.Add(parameter)
	projection, ok := (&generator{}).parseProxyApply(parameters, []model.Expression{call}, body)
	require.True(t, ok)

	traversal, ok := projection.(*model.RelativeTraversalExpression)
	require.True(t, ok)
	assert.Same(t, call, traversal.Source)
	require.Len(t, traversal.Traversal, 1)
	attr, ok := traversal.Traversal[0].(hcl.TraverseAttr)
	require.True(t, ok)
	assert.Equal(t, "result", attr.Name)
	assert.True(t, traversal.Type().Equals(model.NewOutputType(model.NewOptionalType(model.StringType))))
}

func TestParseProxyApplyWithOutputMemberName(t *testing.T) {
	t.Parallel()

	resultType := model.NewObjectType(map[string]model.Type{
		"apply": model.StringType,
	})
	call := &model.FunctionCallExpression{
		Name: "call",
		Signature: model.StaticFunctionSignature{
			ReturnType: model.NewOutputType(resultType),
		},
	}
	parameter := &model.Variable{Name: "result", VariableType: resultType}
	body := &model.RelativeTraversalExpression{
		Source: model.VariableReference(parameter),
		Traversal: hcl.Traversal{
			hcl.TraverseAttr{Name: "apply"},
		},
	}
	require.Empty(t, body.Typecheck(false))

	parameters := mapset.NewSet[model.Traversable]()
	parameters.Add(parameter)
	_, ok := (&generator{}).parseProxyApply(parameters, []model.Expression{call}, body)
	assert.False(t, ok)
}
