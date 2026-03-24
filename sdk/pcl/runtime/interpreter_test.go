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

package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type mockLoader struct{}

func (*mockLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	if pkg != "test" {
		return nil, errors.New("unknown package: " + pkg)
	}

	spec := schema.PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"test:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{Type: "boolean"},
						},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": {
						TypeSpec: schema.TypeSpec{Type: "boolean"},
					},
				},
			},
		},
	}
	p, diags, err := schema.BindSpec(spec, nil, schema.ValidationOptions{})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return p, nil
}

func (l *mockLoader) LoadPackageV2(ctx context.Context, descriptor *schema.PackageDescriptor) (*schema.Package, error) {
	return l.LoadPackage(descriptor.PackageName(), descriptor.PackageVersion())
}

// TestPoisonPropagatesThroughPCLAttributeAccess parses a PCL program similar to
// l2-failed-create-continue-on-error and verifies that when a resource variable is poisoned
// (simulating a failed create), evaluating an attribute reference like `failing.value` on the
// poisoned resource correctly detects the poison mark.
//
// This is a regression test: HCL's GetAttr function drops cty marks when operating on
// DynamicPseudoType values. Without the fix in model.ScopeTraversalExpression.Evaluate,
// the poison mark is silently lost, causing the dependent resource to attempt registration
// with unknown values instead of being skipped.
func TestPoisonPropagatesThroughPCLAttributeAccess(t *testing.T) {
	t.Parallel()

	// This program mirrors the l2-failed-create-continue-on-error test.
	// "failing" will be poisoned, and "dependent_on_output" references failing.value.
	const program = `
resource "failing" "test:index:Resource" {
    value = false
}

resource "dependent_on_output" "test:index:Resource" {
    value = failing.value
}
`

	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(program), "test.pp")
	require.NoError(t, err)
	require.False(t, parser.Diagnostics.HasErrors(), parser.Diagnostics.Error())

	bound, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(&mockLoader{}))
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), diags.Error())

	// Find the "dependent_on_output" resource and its "value" input expression.
	var depExpr *pcl.Resource
	for _, node := range bound.Nodes {
		if r, ok := node.(*pcl.Resource); ok && r.Name() == "dependent_on_output" {
			depExpr = r
			break
		}
	}
	require.NotNil(t, depExpr, "could not find dependent_on_output resource in bound program")

	var valueExpr model.Expression
	for _, attr := range depExpr.Inputs {
		if attr.Name == "value" {
			valueExpr = attr.Value
			break
		}
	}
	require.NotNil(t, valueExpr, "could not find 'value' input on dependent_on_output")

	// Simulate a failed "failing" resource by setting its variable to a poison value.
	poisoned := makePoisonValue("failing")
	evalCtx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"failing": poisoned,
		},
	}

	// Evaluate the expression "failing.value". The poison mark must survive the traversal.
	result, diags := valueExpr.Evaluate(evalCtx)
	require.False(t, diags.HasErrors(), diags.Error())

	// ctyToPropertyValue must detect the poison and return a poisonError.
	_, convErr := ctyToPropertyValue(result)
	var poison *poisonError
	assert.ErrorAs(t, convErr, &poison, "poison mark must survive attribute access on a poisoned resource")
	if poison != nil {
		assert.Equal(t, "failing", poison.name)
	}
}
