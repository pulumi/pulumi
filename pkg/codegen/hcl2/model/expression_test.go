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

package model

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/text/unicode/norm"
	"pgregory.net/rapid"
)

func TestEscapeStringRoundTrip(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")

		expr := &TemplateExpression{Parts: []Expression{
			&LiteralValueExpression{Value: cty.StringVal(s)},
		}}

		hclText := fmt.Sprintf("%v", expr)
		parsed, diags := hclsyntax.ParseExpression([]byte(hclText), "test", hcl.Pos{Line: 1, Column: 1})
		require.False(t, diags.HasErrors(), "failed to parse %q: %s", hclText, diags.Error())

		val, valDiags := parsed.Value(nil)
		require.Empty(t, valDiags.HasErrors(), "failed to evaluate %q: %s", hclText, valDiags.Error())

		// HCL applies Unicode NFC normalization to string values, so we
		// compare against the NFC-normalized input.
		require.Equal(t, norm.NFC.String(s), val.AsString())
	})
}

func TestBindIntegerLiterals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		expr     string
		expected Type
	}{
		{expr: "0", expected: IntType},
		{expr: "2147483647", expected: IntType},
		{expr: "2147483648", expected: NumberType},
		{expr: "-2147483648", expected: IntType},
		{expr: "-2147483649", expected: NumberType},
		{expr: "3.14", expected: NumberType},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()

			expr, diags := BindExpressionText(tc.expr, nil, hcl.Pos{})
			require.False(t, diags.HasErrors(), "failed to bind %q: %s", tc.expr, diags.Error())

			constType, ok := expr.Type().(*ConstType)
			require.True(t, ok, "expected const type for %q, got %T", tc.expr, expr.Type())
			require.True(t, constType.Type.Equals(tc.expected),
				"expected %v for %q, got %v", tc.expected, tc.expr, constType.Type)
		})
	}
}
