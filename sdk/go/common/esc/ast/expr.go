// Copyright 2023, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc/syntax"
)

// Expr represents a Pulumi YAML expression. Expressions may be literals, interpolated strings, symbols, or builtin
// functions.
type Expr interface {
	Node

	Syntax() syntax.Node

	isExpr()
}

type exprNode struct {
	syntax syntax.Node
}

func expr(node syntax.Node) exprNode {
	return exprNode{syntax: node}
}

func (*exprNode) isNode() {}

func (*exprNode) isExpr() {}

func (x *exprNode) Syntax() syntax.Node {
	if x == nil {
		return nil
	}
	return x.syntax
}

// ExprError creates an error-level diagnostic associated with the given expression. If the expression is non-nil and
// has an underlying syntax node, the error will cover the underlying textual range.
func ExprError(expr Expr, summary string) *syntax.Diagnostic {
	var rng *hcl.Range
	if expr != nil {
		if syntax := expr.Syntax(); syntax != nil {
			rng = syntax.Syntax().Range()
		}
	}
	return syntax.Error(rng, summary, expr.Syntax().Syntax().Path())
}

// A NullExpr represents a null literal.
type NullExpr struct {
	exprNode
}

// NullSyntax creates a new null literal expression with associated syntax.
func NullSyntax(node *syntax.NullNode) *NullExpr {
	return &NullExpr{exprNode: expr(node)}
}

// Null creates a new null literal expression.
func Null() *NullExpr {
	return NullSyntax(syntax.Null())
}

// A BooleanExpr represents a boolean literal.
type BooleanExpr struct {
	exprNode

	Value bool
}

// BooleanSyntax creates a new boolean literal expression with the given value and associated syntax.
func BooleanSyntax(node *syntax.BooleanNode) *BooleanExpr {
	return &BooleanExpr{exprNode: expr(node), Value: node.Value()}
}

// Boolean creates a new boolean literal expression with the given value.
func Boolean(value bool) *BooleanExpr {
	return BooleanSyntax(syntax.Boolean(value))
}

// A NumberExpr represents a number literal.
type NumberExpr struct {
	exprNode

	Value json.Number
}

// NumberSyntax creates a new number literal expression with the given value and associated syntax.
func NumberSyntax(node *syntax.NumberNode) *NumberExpr {
	return &NumberExpr{exprNode: expr(node), Value: node.Value()}
}

// Number creates a new number literal expression with the given value.
func Number[T syntax.NumberValue](value T) *NumberExpr {
	return NumberSyntax(syntax.Number(value))
}

// A StringExpr represents a string literal.
type StringExpr struct {
	exprNode

	Value string
}

// GetValue returns the expression's value. If the receiver is null, GetValue returns the empty string.
func (x *StringExpr) GetValue() string {
	if x == nil {
		return ""
	}
	return x.Value
}

// StringSyntax creates a new string literal expression with the given value and associated syntax.
func StringSyntax(node *syntax.StringNode) *StringExpr {
	return &StringExpr{exprNode: expr(node), Value: node.Value()}
}

// StringSyntaxValue creates a new string literal expression with the given syntax and value.
func StringSyntaxValue(node *syntax.StringNode, value string) *StringExpr {
	return &StringExpr{exprNode: expr(node), Value: value}
}

// String creates a new string literal expression with the given value.
func String(value string) *StringExpr {
	return &StringExpr{Value: value}
}

// An InterpolateExpr represents an interpolated string.
//
// Interpolated strings are represented syntactically as strings of the form "some text with ${property.accesses}".
// During evaluation, each access replaced with its evaluated value coerced to a string.
//
// In order to allow convenient access to object properties without string coercion, a string of the form
// "${property.access}" is parsed as a symbol rather than an interpolated string.
type InterpolateExpr struct {
	exprNode

	Parts []Interpolation
}

func (n *InterpolateExpr) String() string {
	var str strings.Builder
	for _, p := range n.Parts {
		// un-escape the string back to its original form
		// this is necessary because the parser will escape the string
		// so when we print it back out as string, we need to un-escape it
		str.WriteString(strings.ReplaceAll(p.Text, "$", "$$"))
		if p.Value != nil {
			fmt.Fprintf(&str, "${%v}", p.Value)
		}
	}
	return str.String()
}

// InterpolateSyntax creates a new interpolated string expression with associated syntax by parsing the given input
// string literal.
func InterpolateSyntax(node *syntax.StringNode) (*InterpolateExpr, syntax.Diagnostics) {
	parts, diags := parseInterpolate(node, node.Value())
	if diags.HasErrors() {
		return nil, diags
	}

	for _, part := range parts {
		if part.Value != nil && len(part.Value.Accessors) == 0 {
			diags.Extend(syntax.NodeError(node, "Property access expressions cannot be empty"))
		}
	}

	return &InterpolateExpr{
		exprNode: expr(node),
		Parts:    parts,
	}, diags
}

// Interpolate creates a new interpolated string expression by parsing the given input string.
func Interpolate(value string) (*InterpolateExpr, syntax.Diagnostics) {
	return InterpolateSyntax(syntax.String(value))
}

// MustInterpolate creates a new interpolated string expression and panics if parsing fails.
func MustInterpolate(value string) *InterpolateExpr {
	x, diags := Interpolate(value)
	if diags.HasErrors() {
		panic(diags)
	}
	return x
}

// A SymbolExpr represents a symbol: a reference to a resource or config property.
//
// Symbol expressions are represented as strings of the form "${resource.property}".
type SymbolExpr struct {
	exprNode

	Property *PropertyAccess
}

// Symbol creates a new
func Symbol(accessors ...PropertyAccessor) *SymbolExpr {
	property := &PropertyAccess{Accessors: accessors}
	return &SymbolExpr{
		exprNode: expr(syntax.String(fmt.Sprintf("${%v}", property))),
		Property: property,
	}
}

func (n *SymbolExpr) String() string {
	return fmt.Sprintf("${%v}", n.Property)
}

// A ArrayExpr represents a list of expressions.
type ArrayExpr struct {
	exprNode

	Elements []Expr
}

// ArraySyntax creates a new list expression with the given elements and associated syntax.
func ArraySyntax(node *syntax.ArrayNode, elements ...Expr) *ArrayExpr {
	return &ArrayExpr{
		exprNode: expr(node),
		Elements: elements,
	}
}

// Array creates a new list expression with the given elements.
func Array(elements ...Expr) *ArrayExpr {
	return ArraySyntax(syntax.Array(), elements...)
}

// An ObjectExpr represents an object.
type ObjectExpr struct {
	exprNode

	Entries []ObjectProperty
}

// An ObjectProperty represents an object property. Key must be a string.
type ObjectProperty struct {
	syntax syntax.ObjectPropertyDef
	Key    *StringExpr
	Value  Expr
}

// ObjectSyntax creates a new object expression with the given properties and associated syntax.
func ObjectSyntax(node *syntax.ObjectNode, entries ...ObjectProperty) *ObjectExpr {
	return &ObjectExpr{
		exprNode: expr(node),
		Entries:  entries,
	}
}

// Object creates a new object expression with the given properties.
func Object(entries ...ObjectProperty) *ObjectExpr {
	return ObjectSyntax(syntax.ObjectSyntax(syntax.NoSyntax), entries...)
}

// ParseExpr parses an expression from the given syntax tree.
//
// The syntax tree is parsed using the following rules:
//
//   - *syntax.{Null,Boolean,Number}Node is parsed as a *{Null,Boolean,Number}Expr.
//   - *syntax.ArrayNode is parsed as a *ArrayExpr.
//   - *syntax.StringNode is parsed as an *InterpolateExpr, a *SymbolExpr, or a *StringExpr. The node's literal is first
//     parsed as an interpolated string. If the result contains a single property access with no surrounding text, (i.e.
//     the string is of the form "${resource.property}", it is treated as a symbol. If the result contains no property
//     accesses, it is treated as a string literal. Otherwise, it it treated as an interpolated string.
//   - *syntax.ObjectNode is parses as either an *ObjectExpr or a BuiltinExpr. If the object contains a single key and
//     that key names a builtin function ("fn::invoke", "fn::join", "fn::select",
//     "fn::*Asset", "fn::*Archive", or "fn::stackReference"), then the object is parsed as the corresponding BuiltinExpr.
//     Otherwise, the object is parsed as a *syntax.ObjectNode.
func ParseExpr(node syntax.Node) (Expr, syntax.Diagnostics) {
	switch node := node.(type) {
	case *syntax.NullNode:
		return NullSyntax(node), nil
	case *syntax.BooleanNode:
		return BooleanSyntax(node), nil
	case *syntax.NumberNode:
		return NumberSyntax(node), nil
	case *syntax.StringNode:
		interpolate, diags := InterpolateSyntax(node)

		if interpolate != nil {
			switch len(interpolate.Parts) {
			case 0:
				return StringSyntax(node), diags
			case 1:
				switch {
				case interpolate.Parts[0].Value == nil:
					return StringSyntaxValue(node, interpolate.Parts[0].Text), diags
				case interpolate.Parts[0].Text == "":
					return &SymbolExpr{
						exprNode: expr(node),
						Property: interpolate.Parts[0].Value,
					}, diags
				}
			}
		}

		return interpolate, diags
	case *syntax.ArrayNode:
		var diags syntax.Diagnostics

		elements := make([]Expr, node.Len())
		for i := range elements {
			x, xdiags := ParseExpr(node.Index(i))
			diags.Extend(xdiags...)
			elements[i] = x
		}
		return ArraySyntax(node, elements...), diags
	case *syntax.ObjectNode:

		var diags syntax.Diagnostics

		x, fnDiags, ok := tryParseFunction(node)
		if ok {
			return x, fnDiags
		}
		diags.Extend(fnDiags...)

		kvps := make([]ObjectProperty, node.Len())
		for i := range kvps {
			kvp := node.Index(i)

			kx, kdiags := ParseExpr(kvp.Key)
			diags.Extend(kdiags...)

			k, ok := kx.(*StringExpr)
			if !ok {
				diags.Extend(syntax.NodeError(kvp.Key, "object keys must be strings"))
			}

			v, vdiags := ParseExpr(kvp.Value)
			diags.Extend(vdiags...)

			kvps[i] = ObjectProperty{syntax: kvp, Key: k, Value: v}
		}
		return ObjectSyntax(node, kvps...), diags
	default:
		return nil, syntax.Diagnostics{syntax.NodeError(node, fmt.Sprintf("unexpected syntax node of type %T", node))}
	}
}

// BuiltinExpr represents a call to a builtin function.
type BuiltinExpr interface {
	Expr

	Name() *StringExpr
	Args() Expr

	isBuiltin()
}

type builtinNode struct {
	exprNode

	name *StringExpr
	args Expr
}

func builtin(node *syntax.ObjectNode, name *StringExpr, args Expr) builtinNode {
	return builtinNode{exprNode: expr(node), name: name, args: args}
}

func (*builtinNode) isBuiltin() {}

func (n *builtinNode) Name() *StringExpr {
	return n.name
}

func (n *builtinNode) Args() Expr {
	return n.args
}

// OpenExpr is a function expression that invokes an environment provider by name.
type OpenExpr struct {
	builtinNode

	Provider *StringExpr
	Inputs   Expr
}

func OpenSyntax(node *syntax.ObjectNode, name *StringExpr, args Expr, provider *StringExpr, inputs Expr) *OpenExpr {
	return &OpenExpr{
		builtinNode: builtin(node, name, args),
		Provider:    provider,
		Inputs:      inputs,
	}
}

func Open(provider string, inputs *ObjectExpr) *OpenExpr {
	name, providerX := String("fn::open"), String(provider)

	entries := []ObjectProperty{
		{Key: String("provider"), Value: providerX},
		{Key: String("inputs"), Value: inputs},
	}

	return &OpenExpr{
		builtinNode: builtin(nil, name, Object(entries...)),
		Provider:    providerX,
		Inputs:      inputs,
	}
}

// ToJSON returns the underlying structure as a json string.
type ToJSONExpr struct {
	builtinNode

	Value Expr
}

func ToJSONSyntax(node *syntax.ObjectNode, name *StringExpr, args Expr) *ToJSONExpr {
	return &ToJSONExpr{
		builtinNode: builtin(node, name, args),
		Value:       args,
	}
}

func ToJSON(value Expr) *ToJSONExpr {
	name := String("fn::toJSON")
	return ToJSONSyntax(nil, name, value)
}

// FromJSON deserializes a JSON string into a value.
type FromJSONExpr struct {
	builtinNode

	String Expr
}

func FromJSONSyntax(node *syntax.ObjectNode, name *StringExpr, args Expr) *FromJSONExpr {
	return &FromJSONExpr{
		builtinNode: builtin(node, name, args),
		String:      args,
	}
}

func FromJSON(value Expr) *FromJSONExpr {
	name := String("fn::fromJSON")
	return FromJSONSyntax(nil, name, value)
}

// ToString returns the underlying structure as a string.
type ToStringExpr struct {
	builtinNode

	Value Expr
}

func ToStringSyntax(node *syntax.ObjectNode, name *StringExpr, args Expr) *ToStringExpr {
	return &ToStringExpr{
		builtinNode: builtin(node, name, args),
		Value:       args,
	}
}

func ToString(value Expr) *ToStringExpr {
	name := String("fn::toString")
	return ToStringSyntax(nil, name, value)
}

// JoinExpr appends a set of values into a single value, separated by the specified delimiter.
// If a delimiter is the empty string, the set of values are concatenated with no delimiter.
type JoinExpr struct {
	builtinNode

	Delimiter Expr
	Values    Expr
}

func JoinSyntax(node *syntax.ObjectNode, name *StringExpr, args *ArrayExpr, delimiter Expr, values Expr) *JoinExpr {
	return &JoinExpr{
		builtinNode: builtin(node, name, args),
		Delimiter:   delimiter,
		Values:      values,
	}
}

func Join(delimiter Expr, values *ArrayExpr) *JoinExpr {
	name := String("fn::join")
	return &JoinExpr{
		builtinNode: builtin(nil, name, Array(delimiter, values)),
		Delimiter:   delimiter,
		Values:      values,
	}
}

type SecretExpr struct {
	builtinNode

	Plaintext  *StringExpr
	Ciphertext *StringExpr
}

func PlaintextSyntax(node *syntax.ObjectNode, name, value *StringExpr) *SecretExpr {
	return &SecretExpr{
		builtinNode: builtin(node, name, value),
		Plaintext:   value,
	}
}

func Plaintext(value *StringExpr) *SecretExpr {
	name := String("fn::secret")

	return &SecretExpr{
		builtinNode: builtin(nil, name, value),
		Plaintext:   value,
	}
}

func CiphertextSyntax(node *syntax.ObjectNode, name *StringExpr, args *ObjectExpr, value *StringExpr) *SecretExpr {
	return &SecretExpr{
		builtinNode: builtin(node, name, args),
		Ciphertext:  value,
	}
}

func Ciphertext(value *StringExpr) *SecretExpr {
	name := String("fn::secret")
	arg := Object(ObjectProperty{Key: String("ciphertext"), Value: value})

	return &SecretExpr{
		builtinNode: builtin(nil, name, arg),
		Ciphertext:  value,
	}
}

type ToBase64Expr struct {
	builtinNode

	Value Expr
}

func ToBase64Syntax(node *syntax.ObjectNode, name *StringExpr, args Expr) *ToBase64Expr {
	return &ToBase64Expr{
		builtinNode: builtin(node, name, args),
		Value:       args,
	}
}

// FromBase64 decodes a Base64 string.
type FromBase64Expr struct {
	builtinNode

	String Expr
}

func FromBase64Syntax(node *syntax.ObjectNode, name *StringExpr, args Expr) *FromBase64Expr {
	return &FromBase64Expr{
		builtinNode: builtin(node, name, args),
		String:      args,
	}
}

func FromBase64(value Expr) *FromBase64Expr {
	name := String("fn::fromBase64")
	return FromBase64Syntax(nil, name, value)
}

func tryParseFunction(node *syntax.ObjectNode) (Expr, syntax.Diagnostics, bool) {
	if node.Len() != 1 {
		return nil, nil, false
	}

	kvp := node.Index(0)

	var parse func(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics)
	var diags syntax.Diagnostics
	switch kvp.Key.Value() {
	case "fn::fromJSON":
		parse = parseFromJSON
	case "fn::fromBase64":
		parse = parseFromBase64
	case "fn::join":
		parse = parseJoin
	case "fn::open":
		parse = parseOpen
	case "fn::secret":
		parse = parseSecret
	case "fn::toBase64":
		parse = parseToBase64
	case "fn::toJSON":
		parse = parseToJSON
	case "fn::toString":
		parse = parseToString
	default:
		if strings.HasPrefix(kvp.Key.Value(), "fn::open::") {
			parse = parseShortOpen
			break
		}

		if strings.HasPrefix(strings.ToLower(kvp.Key.Value()), "fn::") {
			diags = append(diags, syntax.Error(kvp.Key.Syntax().Range(),
				"'fn::' is a reserved prefix",
				node.Syntax().Path()))
		}
		return nil, diags, false
	}

	name := StringSyntax(kvp.Key)

	args, adiags := ParseExpr(kvp.Value)
	diags.Extend(adiags...)

	expr, xdiags := parse(node, name, args)
	diags.Extend(xdiags...)

	if expr == nil {
		expr = ObjectSyntax(node, ObjectProperty{
			syntax: kvp,
			Key:    name,
			Value:  args,
		})
	}

	return expr, diags, true
}

func parseOpen(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	obj, ok := args.(*ObjectExpr)
	if !ok {
		return nil, syntax.Diagnostics{ExprError(args, "the argument to fn::open must be an object containing 'provider' and 'inputs'")}
	}

	var providerExpr, inputs Expr
	var diags syntax.Diagnostics

	for i := 0; i < len(obj.Entries); i++ {
		kvp := obj.Entries[i]
		key := kvp.Key
		switch key.GetValue() {
		case "provider":
			providerExpr = kvp.Value
		case "inputs":
			inputs = kvp.Value
		}
	}

	provider, ok := providerExpr.(*StringExpr)
	if !ok {
		if providerExpr == nil {
			diags.Extend(ExprError(obj, "missing provider name ('provider')"))
		} else {
			diags.Extend(ExprError(providerExpr, "provider name must be a string literal"))
		}
	}

	if inputs == nil {
		diags.Extend(ExprError(obj, "missing provider inputs ('inputs')"))
	}

	if diags.HasErrors() {
		return nil, diags
	}

	return OpenSyntax(node, name, obj, provider, inputs), diags
}

func parseShortOpen(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	kvp := node.Index(0)
	provider := strings.TrimPrefix(kvp.Key.Value(), "fn::open::")
	if args == nil {
		return nil, syntax.Diagnostics{ExprError(name, "missing provider inputs")}
	}
	p := name.Syntax().(*syntax.StringNode)

	return OpenSyntax(node, name, args, StringSyntaxValue(p, provider), args), nil
}

func parseJoin(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	list, ok := args.(*ArrayExpr)
	if !ok || len(list.Elements) != 2 {
		return nil, syntax.Diagnostics{ExprError(args, "the argument to fn::join must be a two-valued list")}
	}

	return JoinSyntax(node, name, list, list.Elements[0], list.Elements[1]), nil
}

func parseToJSON(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	return ToJSONSyntax(node, name, args), nil
}

func parseFromJSON(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	return FromJSONSyntax(node, name, args), nil
}

func parseToString(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	return ToStringSyntax(node, name, args), nil
}

func parseToBase64(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	return ToBase64Syntax(node, name, args), nil
}

func parseFromBase64(node *syntax.ObjectNode, name *StringExpr, args Expr) (Expr, syntax.Diagnostics) {
	return FromBase64Syntax(node, name, args), nil
}

func parseSecret(node *syntax.ObjectNode, name *StringExpr, value Expr) (Expr, syntax.Diagnostics) {
	if arg, ok := value.(*ObjectExpr); ok && len(arg.Entries) == 1 {
		kvp := arg.Entries[0]
		if kvp.Key.Value == "ciphertext" {
			if str, ok := kvp.Value.(*StringExpr); ok {
				return CiphertextSyntax(node, name, arg, str), nil
			}
		}
	}

	str, ok := value.(*StringExpr)
	if !ok {
		return nil, syntax.Diagnostics{ExprError(value, "secret values must be string literals")}
	}
	return PlaintextSyntax(node, name, str), nil
}
