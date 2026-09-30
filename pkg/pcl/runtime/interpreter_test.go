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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
	p, diags, err := schema.BindSpec(spec, schema.NewNullLoader(), schema.ValidationOptions{})
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

	bound, diags, err := pcl.BindProgram(parser.Files, &mockLoader{})
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

func TestApplySchemaInputs_Defaults(t *testing.T) {
	t.Parallel()

	properties := []*schema.Property{
		{
			Name:         "boolean",
			DefaultValue: &schema.DefaultValue{Value: false},
		},
		{
			Name:         "numberArray",
			DefaultValue: &schema.DefaultValue{Value: []any{0.0}},
		},
		{
			Name:         "booleanMap",
			DefaultValue: &schema.DefaultValue{Value: map[string]any{"default": false}},
		},
	}

	inputs := property.NewMap(map[string]property.Value{
		"boolean": property.New(true),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)

	assert.Equal(t, property.New(true), converted.Get("boolean"))
	assert.Equal(t, property.New([]property.Value{
		property.New(0.0),
	}), converted.Get("numberArray"))
	assert.Equal(t, property.New(map[string]property.Value{
		"default": property.New(false),
	}), converted.Get("booleanMap"))
}

func TestApplySchemaInputs_Conversions(t *testing.T) {
	t.Parallel()

	nested := &schema.ObjectType{
		Properties: []*schema.Property{
			{Name: "count", Type: schema.IntType},
		},
	}

	properties := []*schema.Property{
		{Name: "boolean", Type: schema.BoolType},
		{Name: "number", Type: schema.NumberType},
		{Name: "integer", Type: schema.IntType},
		{Name: "string", Type: schema.StringType},
		{Name: "numbers", Type: &schema.ArrayType{ElementType: schema.NumberType}},
		{Name: "flags", Type: &schema.MapType{ElementType: schema.BoolType}},
		{Name: "nested", Type: nested},
	}

	inputs := property.NewMap(map[string]property.Value{
		// String "44" coerces to number 44 when the schema says number.
		"number":  property.New("44"),
		"integer": property.New("7"),
		// String "true" coerces to bool.
		"boolean": property.New("true"),
		// Number coerces to its decimal string.
		"string": property.New(3.5),
		// Array elements coerce per the element type.
		"numbers": property.New([]property.Value{
			property.New("1"),
			property.New("2.5"),
		}),
		// Map values coerce per the element type.
		"flags": property.New(map[string]property.Value{
			"a": property.New("true"),
			"b": property.New("false"),
		}),
		// Nested object fields coerce too.
		"nested": property.New(map[string]property.Value{
			"count": property.New("99"),
		}),
		// Properties not in the schema pass through unchanged.
		"extra": property.New("untouched"),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)

	assert.Equal(t, property.New(true), converted.Get("boolean"))
	assert.Equal(t, property.New(44.0), converted.Get("number"))
	assert.Equal(t, property.New(7.0), converted.Get("integer"))
	assert.Equal(t, property.New("3.5"), converted.Get("string"))
	assert.Equal(t, property.New([]property.Value{
		property.New(1.0),
		property.New(2.5),
	}), converted.Get("numbers"))
	assert.Equal(t, property.New(map[string]property.Value{
		"a": property.New(true),
		"b": property.New(false),
	}), converted.Get("flags"))
	assert.Equal(t, property.New(map[string]property.Value{
		"count": property.New(99.0),
	}), converted.Get("nested"))
	assert.Equal(t, property.New("untouched"), converted.Get("extra"))
}

func TestApplySchemaInputs_PreservesSecrets(t *testing.T) {
	t.Parallel()

	properties := []*schema.Property{
		{Name: "count", Type: schema.IntType},
	}

	inputs := property.NewMap(map[string]property.Value{
		"count": property.New("42").WithSecret(true),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)
	assert.Equal(t, property.New(42.0).WithSecret(true), converted.Get("count"))
}

func TestApplySchemaInputs_Secrets(t *testing.T) {
	t.Parallel()

	properties := []*schema.Property{
		{Name: "token", Type: schema.StringType, Secret: true},
		{Name: "name", Type: schema.StringType},
		{Name: "preMarked", Type: schema.StringType, Secret: true},
		{Name: "missing", Type: schema.StringType, Secret: true},
	}

	inputs := property.NewMap(map[string]property.Value{
		"token":     property.New("s3cret"),
		"name":      property.New("hello"),
		"preMarked": property.New("already").WithSecret(true),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)

	assert.Equal(t, property.New("s3cret").WithSecret(true), converted.Get("token"))
	// Non-secret property is left alone.
	assert.Equal(t, property.New("hello"), converted.Get("name"))
	// Already-secret property isn't double-wrapped.
	assert.Equal(t, property.New("already").WithSecret(true), converted.Get("preMarked"))
	// Missing properties without defaults are not synthesized, even if marked secret.
	_, has := converted.GetOk("missing")
	assert.False(t, has)
}

func TestApplySchemaInputs_RecursesIntoNestedObjects(t *testing.T) {
	t.Parallel()

	// Nested type with both a default and a secret property.
	data := &schema.ObjectType{
		Properties: []*schema.Property{
			{Name: "public", Type: schema.StringType},
			{Name: "private", Type: schema.StringType, Secret: true},
			{
				Name:         "region",
				Type:         schema.StringType,
				DefaultValue: &schema.DefaultValue{Value: "us-west-2"},
			},
		},
	}

	properties := []*schema.Property{
		// Outer property is itself secret — nested marking still happens, even though the
		// outer wrap arguably already covers everything.
		{Name: "outerSecret", Type: data, Secret: true},
		// Outer property is not secret — nested secrets must be marked.
		{Name: "outer", Type: data},
	}

	inputs := property.NewMap(map[string]property.Value{
		"outerSecret": property.New(map[string]property.Value{
			"public":  property.New("o"),
			"private": property.New("p"),
		}),
		"outer": property.New(map[string]property.Value{
			"public":  property.New("o"),
			"private": property.New("p"),
		}),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)

	// outer (not secret-wrapped at top level): nested private is marked, region default fills in.
	assert.Equal(t, property.New(map[string]property.Value{
		"public":  property.New("o"),
		"private": property.New("p").WithSecret(true),
		"region":  property.New("us-west-2"),
	}), converted.Get("outer"))

	// outerSecret: the whole thing is wrapped, defaults still fill in inside, but the
	// schema-driven secret mark on "private" is suppressed because the outer wrap already
	// covers everything.
	assert.Equal(t, property.New(map[string]property.Value{
		"public":  property.New("o"),
		"private": property.New("p"),
		"region":  property.New("us-west-2"),
	}).WithSecret(true), converted.Get("outerSecret"))
}

func TestApplySchemaInputs_PreservesUserSecretInsideSecretParent(t *testing.T) {
	t.Parallel()

	data := &schema.ObjectType{
		Properties: []*schema.Property{
			{Name: "private", Type: schema.StringType, Secret: true},
			{Name: "public", Type: schema.StringType},
		},
	}
	properties := []*schema.Property{
		{Name: "outer", Type: data, Secret: true},
	}

	// User explicitly marked the inner "private" as secret. The outer wrap shouldn't strip it.
	inputs := property.NewMap(map[string]property.Value{
		"outer": property.New(map[string]property.Value{
			"private": property.New("user-marked").WithSecret(true),
			"public":  property.New("p"),
		}),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)

	assert.Equal(t, property.New(map[string]property.Value{
		"private": property.New("user-marked").WithSecret(true),
		"public":  property.New("p"),
	}).WithSecret(true), converted.Get("outer"))
}

func TestApplySchemaInputs(t *testing.T) {
	t.Parallel()

	properties := []*schema.Property{
		{Name: "count", Type: schema.IntType},
		{Name: "token", Type: schema.StringType, Secret: true},
		{Name: "region", Type: schema.StringType, DefaultValue: &schema.DefaultValue{Value: "us-west-2"}},
		{
			Name:         "secretWithDefault",
			Type:         schema.StringType,
			Secret:       true,
			DefaultValue: &schema.DefaultValue{Value: "fallback"},
		},
	}

	inputs := property.NewMap(map[string]property.Value{
		"count": property.New("12"),
		"token": property.New("plain"),
	})

	converted, err := applySchemaInputs(inputs, properties)
	require.NoError(t, err)

	assert.Equal(t, property.New(12.0), converted.Get("count"))
	assert.Equal(t, property.New("plain").WithSecret(true), converted.Get("token"))
	assert.Equal(t, property.New("us-west-2"), converted.Get("region"))
	// Defaults that fill in for secret properties get wrapped too.
	assert.Equal(t, property.New("fallback").WithSecret(true), converted.Get("secretWithDefault"))
}

func TestFillSchemaOutputs_MissingSecret(t *testing.T) {
	t.Parallel()

	properties := []*schema.Property{
		{Name: "secretOutput", Type: schema.StringType, Secret: true},
	}

	previewOutputs := fillSchemaOutputs(property.Map{}, properties, true)
	assert.Equal(t, property.New(property.Computed).WithSecret(true), previewOutputs.Get("secretOutput"))

	updateOutputs := fillSchemaOutputs(property.Map{}, properties, false)
	assert.Equal(t, property.New(property.Null).WithSecret(true), updateOutputs.Get("secretOutput"))
}
