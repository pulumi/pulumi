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

package pcl_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

// multiArgInvoke binds a program containing a single local variable `result` whose value is an
// invoke of the multiArgumentInputs function `multiarg:index:funcWithMultiArgs`, and returns the
// bound invoke call expression along with the binding diagnostics.
func multiArgInvoke(t *testing.T, invokeExpr string) (*model.FunctionCallExpression, hcl.Diagnostics) {
	t.Helper()
	source := "result = " + invokeExpr + "\n"
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	if diags.HasErrors() {
		// On binding errors BindProgram returns the diagnostics as the error too; surface the
		// diagnostics to the caller rather than failing here.
		return nil, diags
	}
	require.NoError(t, err)

	require.Len(t, program.Nodes, 1, "expected a single node")
	local, ok := program.Nodes[0].(*pcl.LocalVariable)
	require.True(t, ok, "expected a local variable, got %T", program.Nodes[0])
	call, ok := local.Definition.Value.(*model.FunctionCallExpression)
	require.True(t, ok, "expected the local's value to be a function call, got %T", local.Definition.Value)
	return call, diags
}

// The object-argument form is rejected for multiArgumentInputs functions.
func TestMultiArgumentInvokeRejectsObjectArgument(t *testing.T) {
	t.Parallel()

	_, diags := multiArgInvoke(t,
		`invoke("multiarg:index:funcWithMultiArgs", { a = "hello", b = "world" })`)

	require.True(t, diags.HasErrors(), "expected the object-argument form to be rejected")
	require.Contains(t, diags.Error(), "must be invoked with positional arguments")
}

// The positional form binds and is normalized to the object-argument form.
func TestMultiArgumentInvokePositional(t *testing.T) {
	t.Parallel()

	call, diags := multiArgInvoke(t,
		`invoke("multiarg:index:funcWithMultiArgs", "hello", "world")`)
	require.False(t, diags.HasErrors(), "unexpected diagnostics: %v", diags)

	// After normalization the call is invoke(token, { a = ..., b = ... }) with no options argument.
	require.Len(t, call.Args, 2)
	requireObjectKeys(t, call.Args[1], []string{"a", "b"})
}

// Only the required input is supplied; the optional input is omitted.
func TestMultiArgumentInvokeOmitsOptionalInput(t *testing.T) {
	t.Parallel()

	call, diags := multiArgInvoke(t,
		`invoke("multiarg:index:funcWithMultiArgs", "hello")`)
	require.False(t, diags.HasErrors(), "unexpected diagnostics: %v", diags)

	require.Len(t, call.Args, 2)
	requireObjectKeys(t, call.Args[1], []string{"a"})
}

// invokeOptions may be passed as a trailing positional argument.
func TestMultiArgumentInvokeWithOptions(t *testing.T) {
	t.Parallel()

	call, diags := multiArgInvoke(t,
		`invoke("multiarg:index:funcWithMultiArgs", "hello", "world", { version = "1.2.3" })`)
	require.False(t, diags.HasErrors(), "unexpected diagnostics: %v", diags)

	// The inputs are collapsed into the object argument and the options object is preserved as the
	// trailing argument.
	require.Len(t, call.Args, 3)
	requireObjectKeys(t, call.Args[1], []string{"a", "b"})
	requireObjectKeys(t, call.Args[2], []string{"version"})
}

// An optional input can be skipped with null in order to supply trailing invokeOptions.
func TestMultiArgumentInvokeWithOptionsAndNullInput(t *testing.T) {
	t.Parallel()

	call, diags := multiArgInvoke(t,
		`invoke("multiarg:index:funcWithMultiArgs", "hello", null, { version = "1.2.3" })`)
	require.False(t, diags.HasErrors(), "unexpected diagnostics: %v", diags)

	require.Len(t, call.Args, 3)
	requireObjectKeys(t, call.Args[1], []string{"a", "b"})
	requireObjectKeys(t, call.Args[2], []string{"version"})
}

// An argument after invokeOptions (i.e. more arguments than inputs plus options) is rejected.
func TestMultiArgumentInvokeRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	_, diags := multiArgInvoke(t,
		`invoke("multiarg:index:funcWithMultiArgs", "hello", "world", { version = "1.2.3" }, "extra")`)

	require.True(t, diags.HasErrors(), "expected an argument after invokeOptions to be rejected")
}

// requireObjectKeys asserts that the expression is an object construction with exactly the given
// literal keys, in order.
func requireObjectKeys(t *testing.T, expr model.Expression, keys []string) {
	t.Helper()
	obj, ok := expr.(*model.ObjectConsExpression)
	require.True(t, ok, "expected an object construction expression, got %T", expr)
	actual := make([]string, 0, len(obj.Items))
	for _, item := range obj.Items {
		lit, ok := item.Key.(*model.LiteralValueExpression)
		require.True(t, ok, "expected a literal key, got %T", item.Key)
		actual = append(actual, lit.Value.AsString())
	}
	require.Equal(t, keys, actual)
}
