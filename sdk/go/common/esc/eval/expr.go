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

package eval

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/ast"
	"github.com/pulumi/esc/schema"
)

const (
	exprDeclared int = iota
	exprEvaluating
	exprDone
)

// An expr represents a single expression in an environment definition.
//
// Each expr holds additional state relative to its corresponding syntax. In particular, an expr holds the expression's
// base value, schema, state, secretness, and its memoized value after evaluation.
type expr struct {
	path string   // The path of the expression, if any. Used when reporting cyclic references.
	repr exprRepr // The expression's representation.
	base *value   // The base value of the expression. This is the imported value the expression overrides, if any.

	schema *schema.Schema // The expression's schema. May not be fully-determined until after evaluation.

	state int // The expression's state.

	secret bool // Whether or not to treat the expression's value as secret.

	value *value // The memoized result of evaluating this expression.
}

// newExpr creates a new expression.
func newExpr(path string, repr exprRepr, s *schema.Schema, base *value) *expr {
	return &expr{path: path, repr: repr, schema: s, base: base}
}

// convertRange converts an HCL2 range to an ESC range.
func convertRange(r *hcl.Range, environment string) esc.Range {
	rng := esc.Range{Environment: environment}
	if r != nil {
		rng.Environment = r.Filename
		rng.Begin = esc.Pos{Line: r.Start.Line, Column: r.Start.Column, Byte: r.Start.Byte}
		rng.End = esc.Pos{Line: r.End.Line, Column: r.End.Column, Byte: r.End.Byte}
	}
	return rng
}

// defRange returns the source range for the expression. If the expression does not have source information, it
// returns a range that only refers to the given environment.
func (x *expr) defRange(environment string) esc.Range {
	return convertRange(x.repr.syntax().Syntax().Syntax().Range(), environment)
}

func exportAccessor(accessor ast.PropertyAccessor) esc.Accessor {
	switch a := accessor.(type) {
	case *ast.PropertyName:
		return esc.Accessor{Key: &a.Name}
	case *ast.PropertySubscript:
		switch index := a.Index.(type) {
		case string:
			return esc.Accessor{Key: &index}
		case int:
			return esc.Accessor{Index: &index}
		}
	}
	panic(fmt.Errorf("invalid property accessor %#v", accessor))
}

// export transforms an expr into its exported, serializable representation.
func (x *expr) export(environment string) esc.Expr {
	var base *esc.Expr
	if x.base != nil {
		b := x.base.def.export(environment)
		base = &b
	}

	ex := esc.Expr{
		Range:  x.defRange(environment),
		Schema: x.schema,
		Base:   base,
	}

	switch repr := x.repr.(type) {
	case *literalExpr:
		switch syntax := x.repr.syntax().(type) {
		case *ast.BooleanExpr:
			ex.Literal = syntax.Value
		case *ast.NumberExpr:
			ex.Literal = syntax.Value
		case *ast.StringExpr:
			ex.Literal = syntax.Value
		}
	case *interpolateExpr:
		interp := make([]esc.Interpolation, len(repr.parts))
		for i, p := range repr.parts {
			var value []esc.PropertyAccessor
			if p.value != nil {
				value = make([]esc.PropertyAccessor, len(p.value.accessors))
				for i, a := range p.value.accessors {
					value[i] = esc.PropertyAccessor{
						Accessor: exportAccessor(a.accessor),
						Value:    a.value.def.defRange(environment),
					}
				}
			}
			interp[i] = esc.Interpolation{
				Text:  p.syntax.Text,
				Value: value,
			}
		}
		ex.Interpolate = interp
	case *symbolExpr:
		value := make([]esc.PropertyAccessor, len(repr.property.accessors))
		for i, a := range repr.property.accessors {
			value[i] = esc.PropertyAccessor{
				Accessor: exportAccessor(a.accessor),
				Value:    a.value.def.defRange(environment),
			}
		}
		ex.Symbol = value
	case *accessExpr:
		accessor := exportAccessor(repr.accessor)
		if _, ok := repr.receiver.def.repr.(*accessExpr); ok {
			ex = repr.receiver.def.export(environment)
			ex.Access.Accessors = append(ex.Access.Accessors, accessor)
		} else {
			ex.Access = &esc.AccessExpr{
				Receiver:  repr.receiver.def.defRange(environment),
				Accessors: []esc.Accessor{accessor},
			}
		}
	case *fromBase64Expr:
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.String().Schema(),
			Arg:       repr.string.export(environment),
		}
	case *fromJSONExpr:
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.Always().Schema(),
			Arg:       repr.string.export(environment),
		}
	case *joinExpr:
		argRange := convertRange(repr.node.Args().Syntax().Syntax().Range(), environment)
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.Tuple(schema.String(), schema.Array().Items(schema.String())).Schema(),
			Arg: esc.Expr{
				Range: argRange,
				List:  []esc.Expr{repr.delimiter.export(environment), repr.values.export(environment)},
			},
		}
	case *openExpr:
		name := repr.node.Name().Value
		if name == "fn::open" {
			ex.Builtin = &esc.BuiltinExpr{
				Name:      name,
				NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
				ArgSchema: schema.Record(map[string]schema.Builder{
					"provider": schema.String(),
					"inputs":   repr.inputSchema,
				}).Schema(),
				Arg: esc.Expr{
					Object: map[string]esc.Expr{
						"provider": repr.provider.export(environment),
						"inputs":   repr.inputs.export(environment),
					},
				},
			}
		} else {
			ex.Builtin = &esc.BuiltinExpr{
				Name:      name,
				NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
				ArgSchema: repr.inputSchema,
				Arg:       repr.inputs.export(environment),
			}
		}
	case *secretExpr:
		var arg esc.Expr
		if repr.plaintext != nil {
			arg = repr.plaintext.export(environment)
		} else {
			arg = esc.Expr{
				Range: convertRange(repr.node.Args().Syntax().Syntax().Range(), environment),
				Object: map[string]esc.Expr{
					"ciphertext": repr.ciphertext.export(environment),
				},
			}
		}
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.Always().Schema(),
			Arg:       arg,
		}
	case *toBase64Expr:
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.String().Schema(),
			Arg:       repr.value.export(environment),
		}
	case *toJSONExpr:
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.Always().Schema(),
			Arg:       repr.value.export(environment),
		}
	case *toStringExpr:
		ex.Builtin = &esc.BuiltinExpr{
			Name:      repr.node.Name().Value,
			NameRange: convertRange(repr.node.Name().Syntax().Syntax().Range(), environment),
			ArgSchema: schema.Always().Schema(),
			Arg:       repr.value.export(environment),
		}
	case *arrayExpr:
		ex.List = make([]esc.Expr, len(repr.elements))
		for i, el := range repr.elements {
			ex.List[i] = el.export(environment)
		}
	case *objectExpr:
		ex.KeyRanges = make(map[string]esc.Range, len(repr.node.Entries))
		for _, kvp := range repr.node.Entries {
			ex.KeyRanges[kvp.Key.Value] = convertRange(kvp.Key.Syntax().Syntax().Range(), environment)
		}

		ex.Object = make(map[string]esc.Expr, len(repr.properties))
		for k, v := range repr.properties {
			ex.Object[k] = v.export(environment)
		}
	default:
		panic(fmt.Sprintf("fatal: invalid expr type %T", repr))
	}

	return ex
}

type propertyAccess struct {
	accessors []*propertyAccessor
}

type propertyAccessor struct {
	accessor ast.PropertyAccessor
	value    *value
}

type interpolation struct {
	syntax ast.Interpolation
	value  *propertyAccess
}

type exprRepr interface {
	syntax() ast.Expr
}

// literalExpr represents a literal value.
type literalExpr struct {
	node ast.Expr
}

func (x *literalExpr) syntax() ast.Expr {
	return x.node
}

// interpolateExpr represents an interpolated string.
type interpolateExpr struct {
	node *ast.InterpolateExpr

	parts []interpolation
}

func (x *interpolateExpr) syntax() ast.Expr {
	return x.node
}

// symbolExpr represents a reference to another value.
type symbolExpr struct {
	node *ast.SymbolExpr

	property *propertyAccess
}

func (x *symbolExpr) syntax() ast.Expr {
	return x.node
}

// accessExpr represents a late-bound property access.
type accessExpr struct {
	node ast.Expr

	receiver *value
	accessor ast.PropertyAccessor
}

func (x *accessExpr) syntax() ast.Expr {
	return x.node
}

// arrayExpr represents an array literal.
type arrayExpr struct {
	node *ast.ArrayExpr

	elements []*expr
}

func (x *arrayExpr) syntax() ast.Expr {
	return x.node
}

// objectExpr represents an object literal.
type objectExpr struct {
	node *ast.ObjectExpr

	properties map[string]*expr
}

func (x *objectExpr) syntax() ast.Expr {
	return x.node
}

// openExpr represents a call to the fn::open builtin.
type openExpr struct {
	node *ast.OpenExpr

	provider *expr
	inputs   *expr

	inputSchema *schema.Schema
}

func (x *openExpr) syntax() ast.Expr {
	return x.node
}

// toJSONExpr represents a call to the fn::toJSON builtin.
type toJSONExpr struct {
	node *ast.ToJSONExpr

	value *expr
}

func (x *toJSONExpr) syntax() ast.Expr {
	return x.node
}

// fromJSONExpr represents a call from the fn::fromJSON builtin.
type fromJSONExpr struct {
	node *ast.FromJSONExpr

	string *expr
}

func (x *fromJSONExpr) syntax() ast.Expr {
	return x.node
}

// toStringExpr represents a call to the fn::toString builtin.
type toStringExpr struct {
	node *ast.ToStringExpr

	value *expr
}

func (x *toStringExpr) syntax() ast.Expr {
	return x.node
}

// joinExpr represents a call to the fn::join builtin.
type joinExpr struct {
	node *ast.JoinExpr

	delimiter *expr
	values    *expr
}

func (x *joinExpr) syntax() ast.Expr {
	return x.node
}

// secretExpr represents a call to the fn::secret builtin.
type secretExpr struct {
	node *ast.SecretExpr

	plaintext  *expr
	ciphertext *expr
}

func (x *secretExpr) syntax() ast.Expr {
	return x.node
}

// toBase64Expr represents a call to the fn::toBase64 builtin.
type toBase64Expr struct {
	node *ast.ToBase64Expr

	value *expr
}

func (x *toBase64Expr) syntax() ast.Expr {
	return x.node
}

// fromBase64Expr represents a call from the fn::fromBase64 builtin.
type fromBase64Expr struct {
	node *ast.FromBase64Expr

	string *expr
}

func (x *fromBase64Expr) syntax() ast.Expr {
	return x.node
}
