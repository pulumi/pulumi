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

package ast

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringBuilder(t *testing.T) {
	t.Parallel()

	s := String("hello")
	assert.Equal(t, "hello", s.Value)
	require.NotNil(t, s.Syntax())
}

func TestStringSyntaxValue(t *testing.T) {
	t.Parallel()

	node := syntax.String("raw-syntax-value")
	s := StringSyntaxValue(node, "overridden-value")
	assert.Equal(t, "overridden-value", s.Value)
	assert.Equal(t, "raw-syntax-value", node.Value())
}

func TestStringGetValue(t *testing.T) {
	t.Parallel()

	s := String("hello")
	assert.Equal(t, "hello", s.GetValue())

	var nilStr *StringExpr
	assert.Equal(t, "", nilStr.GetValue())
}

func TestInterpolate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantParts int
		wantErr   bool
	}{
		{"plain string", "hello world", 1, false},
		{"single interpolation", "${foo.bar}", 1, false},
		{"mixed text and interpolation", "prefix-${foo.bar}-suffix", 2, false},
		{"multiple interpolations", "${a.b} and ${c.d}", 2, false},
		{"escaped dollar", "$${not-interpolated}", 1, false},
		{"unclosed interpolation", "${foo", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expr, diags := Interpolate(tt.input)
			if tt.wantErr {
				assert.True(t, diags.HasErrors())
			} else {
				assert.False(t, diags.HasErrors())
			}
			require.Len(t, expr.Parts, tt.wantParts)
		})
	}
}

func TestMustInterpolateValid(t *testing.T) {
	t.Parallel()

	expr := MustInterpolate("hello ${foo.bar}")
	require.NotNil(t, expr)
	require.Len(t, expr.Parts, 1)
	assert.Equal(t, "hello ", expr.Parts[0].Text)
	require.NotNil(t, expr.Parts[0].Value)
}

func TestMustInterpolatePanics(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		MustInterpolate("${foo")
	})
}

func TestInterpolateExprString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"single access", "${foo.bar}", "${foo.bar}"},
		{"mixed", "prefix-${foo.bar}-suffix", "prefix-${foo.bar}-suffix"},
		{"escaped dollar in text", "$${literal}", "$${literal}"},
		{"multiple accesses", "${a.b} and ${c.d}", "${a.b} and ${c.d}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expr, diags := Interpolate(tt.input)
			assert.False(t, diags.HasErrors())
			assert.Equal(t, tt.want, expr.String())
		})
	}
}

func TestSymbolExprString(t *testing.T) {
	t.Parallel()

	sym := Symbol(&PropertyName{Name: "foo"}, &PropertyName{Name: "bar"})
	assert.Equal(t, "${foo.bar}", sym.String())

	sym2 := Symbol(&PropertyName{Name: "root"}, &PropertySubscript{Index: 0}, &PropertyName{Name: "nested"})
	assert.Equal(t, "${root[0].nested}", sym2.String())

	sym3 := Symbol(&PropertyName{Name: "root"}, &PropertySubscript{Index: "key with dots"})
	assert.Equal(t, `${root["key with dots"]}`, sym3.String())
}

func TestParseExprNull(t *testing.T) {
	t.Parallel()

	node := syntax.Null()
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	_, ok := expr.(*NullExpr)
	assert.True(t, ok)
}

func TestParseExprBoolean(t *testing.T) {
	t.Parallel()

	for _, val := range []bool{true, false} {
		node := syntax.Boolean(val)
		expr, diags := ParseExpr(node)
		assert.False(t, diags.HasErrors())
		b, ok := expr.(*BooleanExpr)
		require.True(t, ok)
		assert.Equal(t, val, b.Value)
	}
}

func TestParseExprNumber(t *testing.T) {
	t.Parallel()

	node := syntax.Number(42)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	n, ok := expr.(*NumberExpr)
	require.True(t, ok)
	assert.Equal(t, "42", n.Value.String())
}

func TestParseExprPlainString(t *testing.T) {
	t.Parallel()

	node := syntax.String("hello world")
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	s, ok := expr.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, "hello world", s.Value)
}

func TestParseExprSymbol(t *testing.T) {
	t.Parallel()

	node := syntax.String("${foo.bar}")
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	sym, ok := expr.(*SymbolExpr)
	require.True(t, ok)
	require.Len(t, sym.Property.Accessors, 2)
	assert.Equal(t, "foo", sym.Property.Accessors[0].(*PropertyName).Name)
	assert.Equal(t, "bar", sym.Property.Accessors[1].(*PropertyName).Name)
}

func TestParseExprInterpolation(t *testing.T) {
	t.Parallel()

	node := syntax.String("prefix-${foo.bar}-suffix")
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	interp, ok := expr.(*InterpolateExpr)
	require.True(t, ok)
	assert.Equal(t, "prefix-", interp.Parts[0].Text)
	require.NotNil(t, interp.Parts[0].Value)
	assert.Equal(t, "-suffix", interp.Parts[1].Text)
}

func TestParseExprArray(t *testing.T) {
	t.Parallel()

	node := syntax.Array(syntax.String("a"), syntax.Number(1), syntax.Boolean(true))
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	arr, ok := expr.(*ArrayExpr)
	require.True(t, ok)
	require.Len(t, arr.Elements, 3)
	_, ok = arr.Elements[0].(*StringExpr)
	assert.True(t, ok)
	_, ok = arr.Elements[1].(*NumberExpr)
	assert.True(t, ok)
	_, ok = arr.Elements[2].(*BooleanExpr)
	assert.True(t, ok)
}

func TestParseExprObject(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("key1"), syntax.String("val1")),
		syntax.ObjectProperty(syntax.String("key2"), syntax.Number(2)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	obj, ok := expr.(*ObjectExpr)
	require.True(t, ok)
	require.Len(t, obj.Entries, 2)
	assert.Equal(t, "key1", obj.Entries[0].Key.Value)
	s, ok := obj.Entries[0].Value.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, "val1", s.Value)
}

func TestParseExprBuiltinFnOpen(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::open"), syntax.Object(
			syntax.ObjectProperty(syntax.String("provider"), syntax.String("test-provider")),
			syntax.ObjectProperty(syntax.String("inputs"), syntax.Object(
				syntax.ObjectProperty(syntax.String("key"), syntax.String("value")),
			)),
		)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	open, ok := expr.(*OpenExpr)
	require.True(t, ok)
	assert.Equal(t, "test-provider", open.Provider.Value)
	require.NotNil(t, open.Inputs)
}

func TestParseExprBuiltinFnSecret(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::secret"), syntax.String("my-secret")),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	secret, ok := expr.(*SecretExpr)
	require.True(t, ok)
	assert.Equal(t, "my-secret", secret.Plaintext.Value)
}

func TestParseExprBuiltinFnJoin(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::join"), syntax.Array(
			syntax.String(","),
			syntax.Array(syntax.String("a"), syntax.String("b")),
		)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	join, ok := expr.(*JoinExpr)
	require.True(t, ok)
	delim, ok := join.Delimiter.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, ",", delim.Value)
}

func TestParseExprBuiltinFnToJSON(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::toJSON"), syntax.String("value")),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	_, ok := expr.(*ToJSONExpr)
	assert.True(t, ok)
}

func TestParseExprBuiltinFnFinal(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::final"), syntax.String("immutable")),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	final, ok := expr.(*FinalExpr)
	require.True(t, ok)
	v, ok := final.Value.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, "immutable", v.Value)
}

func TestParseExprBuiltinReservedPrefix(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::unknown"), syntax.String("value")),
	)
	_, diags := ParseExpr(node)
	assert.True(t, diags.HasErrors())
}

func TestParseExprBuiltinMultipleKeysWithFn(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::open"), syntax.String("value")),
		syntax.ObjectProperty(syntax.String("extra"), syntax.String("value")),
	)
	_, diags := ParseExpr(node)
	assert.True(t, diags.HasErrors())
}

func TestParseExprShortOpen(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::open::my-provider"), syntax.Object(
			syntax.ObjectProperty(syntax.String("key"), syntax.String("value")),
		)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	open, ok := expr.(*OpenExpr)
	require.True(t, ok)
	assert.Equal(t, "my-provider", open.Provider.Value)
}

func TestExprErrorWithVariousTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr Expr
	}{
		{"nil StringExpr", (*StringExpr)(nil)},
		{"nil NullExpr", (*NullExpr)(nil)},
		{"nil BooleanExpr", (*BooleanExpr)(nil)},
		{"non-nil StringExpr", String("test")},
		{"non-nil NullExpr", Null()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ExprError(tt.expr, "test error")
			assert.Equal(t, "test error", err.Summary)
		})
	}
}

func TestParseExprEscapedDollar(t *testing.T) {
	t.Parallel()

	node := syntax.String("$${not-interp}")
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	s, ok := expr.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, "${not-interp}", s.Value)
}

func TestParseExprEmptyString(t *testing.T) {
	t.Parallel()

	node := syntax.String("")
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	s, ok := expr.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, "", s.Value)
}

func TestParseExprBuiltinFnConcat(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::concat"), syntax.Array(
			syntax.Array(syntax.String("a")),
			syntax.Array(syntax.String("b")),
		)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	_, ok := expr.(*ConcatExpr)
	assert.True(t, ok)
}

func TestParseExprBuiltinFnSplit(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::split"), syntax.Array(
			syntax.String(","),
			syntax.String("a,b,c"),
		)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	split, ok := expr.(*SplitExpr)
	require.True(t, ok)
	delim, ok := split.Delimiter.(*StringExpr)
	require.True(t, ok)
	assert.Equal(t, ",", delim.Value)
}

func TestParseExprBuiltinFnValidate(t *testing.T) {
	t.Parallel()

	node := syntax.Object(
		syntax.ObjectProperty(syntax.String("fn::validate"), syntax.Object(
			syntax.ObjectProperty(syntax.String("schema"), syntax.Object(
				syntax.ObjectProperty(syntax.String("type"), syntax.String("string")),
			)),
			syntax.ObjectProperty(syntax.String("value"), syntax.String("test")),
		)),
	)
	expr, diags := ParseExpr(node)
	assert.False(t, diags.HasErrors())
	v, ok := expr.(*ValidateExpr)
	require.True(t, ok)
	require.NotNil(t, v.Schema)
	require.NotNil(t, v.Value)
}
