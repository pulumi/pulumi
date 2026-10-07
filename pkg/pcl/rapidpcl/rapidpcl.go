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

// Package rapidpcl provides a rapid (property-based) generator for PCL programs.
//
// A generated program consists of local variables and stack outputs. Every value is an
// expression tree over the locals that precede it. The tree is typed by construction: each
// node records the [model.Type] the PCL binder assigns to it, and each node only composes
// children of the types its form accepts. The generator does not evaluate expressions; the
// PCL interpreter is the reference for the values a program produces.
//
// Built-ins that need a file on disk (readFile, filebase64, filebase64sha256, fileAsset,
// fileArchive), a resource monitor (getOutput, invoke, call, recover, pulumiResourceType,
// pulumiResourceName), or that fail on purpose (notImplemented) are not generated.
// singleOrNone and the asset built-ins are not generated yet.
package rapidpcl

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// Sample is a generated PCL program.
type Sample struct {
	// Source is the text of the program's single main.pp file.
	Source string
	// Providers are the providers the program requires. Programs do not use resources
	// yet, so this is always nil.
	Providers []plugin.Provider
	// Outputs maps each stack output name to the type the generator assigned to its
	// expression. The binder's type for the output converts to it.
	Outputs map[string]model.Type
}

const maxDepth = 3

// Program returns a generator for PCL programs made of local variables and stack outputs.
func Program() *rapid.Generator[*Sample] {
	return rapid.Custom(func(t *rapid.T) *Sample {
		g := &gen{t: t}
		var src strings.Builder
		for i := range rapid.IntRange(0, 3).Draw(t, "locals") {
			name := fmt.Sprintf("v%d", i)
			e := g.expr(g.drawType(2), maxDepth)
			fmt.Fprintf(&src, "%s = %s\n\n", name, e.src)
			g.scope = append(g.scope, binding{name: name, expr: e.ref(name)})
		}
		outputs := map[string]model.Type{}
		for i := range rapid.IntRange(1, 3).Draw(t, "outputs") {
			name := fmt.Sprintf("o%d", i)
			e := g.expr(g.drawType(2), maxDepth)
			fmt.Fprintf(&src, "output %q {\n  value = %s\n}\n\n", name, e.src)
			outputs[name] = e.typ
		}
		return &Sample{Source: src.String(), Outputs: outputs}
	})
}

// expr is a generated expression together with the static facts the generator relies on
// to compose it without introducing a runtime failure.
type expr struct {
	src string
	typ model.Type
	// minLen is a lower bound on the length of a list-typed expression.
	minLen int
	// keys are keys known to be present in a map-typed expression.
	keys []string
}

// ref returns the expression a variable reference to e produces.
func (e expr) ref(name string) expr {
	e.src = name
	return e
}

// wrap returns e with its source replaced and every other fact kept.
func (e expr) wrap(format string, args ...any) expr {
	e.src = fmt.Sprintf(format, args...)
	return e
}

type binding struct {
	name string
	expr expr
}

type gen struct {
	t     *rapid.T
	scope []binding
	names int
}

func (g *gen) fresh(prefix string) string {
	g.names++
	return fmt.Sprintf("%s%d", prefix, g.names)
}

func (g *gen) draw(label string, options []func() expr) expr {
	return rapid.SampledFrom(options).Draw(g.t, label)()
}

// with runs f with name bound to e in scope.
func (g *gen) with(name string, e expr, f func() expr) expr {
	g.scope = append(g.scope, binding{name: name, expr: e})
	defer func() { g.scope = g.scope[:len(g.scope)-1] }()
	return f()
}

var scalarTypes = []model.Type{model.BoolType, model.NumberType, model.StringType}

func (g *gen) drawType(depth int) model.Type {
	if depth == 0 {
		return rapid.SampledFrom(scalarTypes).Draw(g.t, "scalar")
	}
	switch rapid.IntRange(0, 5).Draw(g.t, "kind") {
	case 0:
		return model.BoolType
	case 1:
		return model.NumberType
	case 2:
		return model.StringType
	case 3:
		return model.NewListType(g.drawType(depth - 1))
	case 4:
		return model.NewMapType(g.drawType(depth - 1))
	default:
		return g.drawObjectType(depth, nil)
	}
}

// drawObjectType draws an object type that includes the given properties.
func (g *gen) drawObjectType(depth int, include map[string]model.Type) *model.ObjectType {
	props := map[string]model.Type{}
	for _, k := range rapid.SliceOfNDistinct(identifier, 0, 3, rapid.ID[string]).Draw(g.t, "keys") {
		props[k] = g.drawType(depth - 1)
	}
	maps.Copy(props, include)
	return model.NewObjectType(props)
}

// homogeneousObjectType draws an object type with at least one property, all of type elem.
// The values of an empty object have no type, so iterating one cannot produce a map of elem.
func (g *gen) homogeneousObjectType(elem model.Type) *model.ObjectType {
	props := map[string]model.Type{}
	for _, k := range rapid.SliceOfNDistinct(identifier, 1, 3, rapid.ID[string]).Draw(g.t, "keys") {
		props[k] = elem
	}
	return model.NewObjectType(props)
}

// hclKeywords are the identifiers HCL reads as something other than a name.
var hclKeywords = map[string]bool{
	"for": true, "in": true, "if": true, "else": true, "endif": true, "endfor": true,
	"null": true, "true": true, "false": true,
}

var identifier = rapid.StringMatching(`[a-z][a-zA-Z0-9]{0,6}`).
	Filter(func(s string) bool { return !hclKeywords[s] })

func sortedKeys(t *model.ObjectType) []string {
	keys := make([]string, 0, len(t.Properties))
	for k := range t.Properties {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// expr draws an expression of type typ with at most depth levels of nesting.
func (g *gen) expr(typ model.Type, depth int) expr {
	options := []func() expr{func() expr { return g.literal(typ, depth-1) }}
	for _, b := range g.scope {
		if b.expr.typ.Equals(typ) {
			options = append(options, func() expr { return b.expr })
		}
	}
	if depth <= 0 {
		return g.draw("leaf", options)
	}
	options = append(options, g.commonForms(typ, depth)...)
	switch typ := typ.(type) {
	case *model.ListType:
		options = append(options, g.listForms(typ, depth)...)
	case *model.MapType:
		options = append(options, g.mapForms(typ, depth)...)
	default:
		switch {
		case typ.Equals(model.BoolType):
			options = append(options, g.boolForms(depth)...)
		case typ.Equals(model.NumberType):
			options = append(options, g.numberForms(depth)...)
		case typ.Equals(model.StringType):
			options = append(options, g.stringForms(depth)...)
		}
	}
	return g.draw("form", options)
}

// literal draws a constructor expression of type typ whose children have at most depth
// levels of nesting.
func (g *gen) literal(typ model.Type, depth int) expr {
	switch typ := typ.(type) {
	case *model.ListType:
		return g.tupleCons(typ, depth)
	case *model.MapType:
		// PCL has no map literal. An object-for over a homogeneous object is the only way to
		// construct a value the binder types as a map.
		obj := g.objectCons(g.homogeneousObjectType(typ.ElementType), depth)
		k, v := g.fresh("k"), g.fresh("w")
		return expr{
			src:  fmt.Sprintf("{for %s, %s in %s : %s => %s}", k, v, obj.src, k, v),
			typ:  typ,
			keys: sortedKeys(obj.typ.(*model.ObjectType)),
		}
	case *model.ObjectType:
		return g.objectCons(typ, depth)
	}
	switch {
	case typ.Equals(model.BoolType):
		return expr{src: strconv.FormatBool(rapid.Bool().Draw(g.t, "bool")), typ: typ}
	case typ.Equals(model.NumberType):
		if rapid.Bool().Draw(g.t, "integral") {
			return expr{src: strconv.Itoa(rapid.IntRange(-1000, 1000).Draw(g.t, "int")), typ: typ}
		}
		return expr{src: fmt.Sprintf("%d.%02d", rapid.IntRange(-100, 100).Draw(g.t, "whole"),
			rapid.IntRange(0, 99).Draw(g.t, "fraction")), typ: typ}
	case typ.Equals(model.StringType):
		return expr{src: Quote(rapid.String().Draw(g.t, "string")), typ: typ}
	}
	panic(fmt.Sprintf("rapidpcl: no literal for type %v", typ))
}

// tupleCons draws a tuple constructor with at least one element. An empty tuple literal has
// no element type; an empty list is reachable through a filtered for expression instead.
func (g *gen) tupleCons(typ *model.ListType, depth int) expr {
	n := rapid.IntRange(1, 3).Draw(g.t, "elements")
	parts := make([]string, n)
	for i := range parts {
		parts[i] = g.expr(typ.ElementType, depth).src
	}
	return expr{src: "[" + strings.Join(parts, ", ") + "]", typ: typ, minLen: n}
}

func (g *gen) objectCons(typ *model.ObjectType, depth int) expr {
	parts := make([]string, 0, len(typ.Properties))
	for _, k := range sortedKeys(typ) {
		parts = append(parts, fmt.Sprintf("%q = %s", k, g.expr(typ.Properties[k], depth).src))
	}
	return expr{src: "{" + strings.Join(parts, ", ") + "}", typ: typ}
}

// nonEmptyList draws a list whose length is known to be at least one.
func (g *gen) nonEmptyList(elem model.Type, depth int) expr {
	l := g.expr(model.NewListType(elem), depth)
	if l.minLen == 0 {
		l = g.tupleCons(model.NewListType(elem), depth)
	}
	return l
}

// failing draws an expression of type typ that binds but fails when evaluated.
func (g *gen) failing(typ model.Type, depth int) expr {
	// The binder rejects a literal index it can prove out of range, so the index is computed.
	options := []func() expr{func() expr {
		e := g.expr(typ, depth)
		return e.wrap("[%s][length([%s])]", e.src, e.src)
	}}
	if typ.Equals(model.StringType) {
		options = append(options, func() expr {
			return expr{src: fmt.Sprintf("fromBase64(%s)", Quote("!"+rapid.String().Draw(g.t, "junk"))), typ: typ}
		})
	}
	return g.draw("failing", options)
}

// commonForms are the forms that produce a value of any type.
func (g *gen) commonForms(typ model.Type, depth int) []func() expr {
	return []func() expr{
		func() expr { e := g.expr(typ, depth-1); return e.wrap("secret(%s)", e.src) },
		func() expr { e := g.expr(typ, depth-1); return e.wrap("unsecret(secret(%s))", e.src) },
		func() expr {
			first := g.failing(typ, depth-1)
			if rapid.Bool().Draw(g.t, "succeeds") {
				first = g.expr(typ, depth-1)
			}
			return expr{src: fmt.Sprintf("try(%s, %s)", first.src, g.expr(typ, depth-1).src), typ: typ}
		},
		func() expr {
			l := g.nonEmptyList(typ, depth-1)
			return expr{src: fmt.Sprintf("element(%s, %d)", l.src, rapid.IntRange(0, 5).Draw(g.t, "index")), typ: typ}
		},
		func() expr {
			l := g.nonEmptyList(typ, depth-1)
			return expr{src: fmt.Sprintf("%s[%d]", l.src, rapid.IntRange(0, l.minLen-1).Draw(g.t, "index")), typ: typ}
		},
		func() expr {
			key := identifier.Draw(g.t, "key")
			obj := g.expr(g.drawObjectType(depth-1, map[string]model.Type{key: typ}), depth-1)
			return expr{src: fmt.Sprintf("%s.%s", obj.src, key), typ: typ}
		},
		func() expr {
			args := g.lookupArgs(typ, depth-1, false)
			return expr{src: fmt.Sprintf("lookup(%s, %s)", args, g.expr(typ, depth-1).src), typ: typ}
		},
		func() expr { return expr{src: fmt.Sprintf("lookup(%s)", g.lookupArgs(typ, depth-1, true)), typ: typ} },
		func() expr {
			return expr{src: fmt.Sprintf("(%s ? %s : %s)", g.expr(model.BoolType, depth-1).src,
				g.expr(typ, depth-1).src, g.expr(typ, depth-1).src), typ: typ}
		},
	}
}

// lookupArgs draws the map and key arguments of a lookup into a map of elem. When present is set, the
// key is known to be in the map. A lookup without a default is an index, which fails on an absent key.
func (g *gen) lookupArgs(elem model.Type, depth int, present bool) string {
	m := g.expr(model.NewMapType(elem), depth)
	if present && len(m.keys) == 0 {
		m = g.literal(model.NewMapType(elem), depth)
	}
	key := identifier.Draw(g.t, "key")
	if len(m.keys) > 0 && (present || rapid.Bool().Draw(g.t, "present")) {
		key = rapid.SampledFrom(m.keys).Draw(g.t, "key")
	}
	return fmt.Sprintf("%s, %q", m.src, key)
}

func (g *gen) boolForms(depth int) []func() expr {
	binary := func(op string, operand model.Type) expr {
		return expr{
			src: fmt.Sprintf("(%s %s %s)", g.expr(operand, depth-1).src, op, g.expr(operand, depth-1).src),
			typ: model.BoolType,
		}
	}
	return []func() expr{
		func() expr { return expr{src: "(!" + g.expr(model.BoolType, depth-1).src + ")", typ: model.BoolType} },
		func() expr { return binary("&&", model.BoolType) },
		func() expr { return binary("||", model.BoolType) },
		func() expr { return binary("<", model.NumberType) },
		func() expr { return binary("<=", model.NumberType) },
		func() expr { return binary(">", model.NumberType) },
		func() expr { return binary(">=", model.NumberType) },
		func() expr { return binary("==", g.drawType(1)) },
		func() expr { return binary("!=", g.drawType(1)) },
		func() expr {
			return expr{src: fmt.Sprintf("can(%s)", g.failing(g.drawType(1), depth-1).src), typ: model.BoolType}
		},
		func() expr {
			return expr{src: fmt.Sprintf("can(%s)", g.expr(g.drawType(1), depth-1).src), typ: model.BoolType}
		},
	}
}

func (g *gen) numberForms(depth int) []func() expr {
	number := func(src string) expr { return expr{src: src, typ: model.NumberType} }
	operand := func() string { return g.expr(model.NumberType, depth-1).src }
	binary := func(op string) func() expr {
		return func() expr { return number(fmt.Sprintf("(%s %s %s)", operand(), op, operand())) }
	}
	// nonZero draws a literal divisor; a drawn expression could evaluate to zero.
	nonZero := func() string {
		return strconv.Itoa(rapid.IntRange(1, 100).Draw(g.t, "divisor") * (2*rapid.IntRange(0, 1).Draw(g.t, "sign") - 1))
	}
	varargs := func(name string) func() expr {
		return func() expr {
			rest := rapid.IntRange(0, 2).Draw(g.t, "rest")
			args := make([]string, 0, rest+1)
			args = append(args, operand())
			for range rest {
				args = append(args, operand())
			}
			return number(fmt.Sprintf("%s(%s)", name, strings.Join(args, ", ")))
		}
	}
	return []func() expr{
		func() expr { return number("(-" + operand() + ")") },
		binary("+"),
		binary("-"),
		binary("*"),
		func() expr { return number(fmt.Sprintf("(%s / %s)", operand(), nonZero())) },
		func() expr { return number(fmt.Sprintf("(%s %% %s)", operand(), nonZero())) },
		func() expr { return number(fmt.Sprintf("length(%s)", g.expr(g.drawCollectionType(1), depth-1).src)) },
		varargs("max"),
		varargs("min"),
	}
}

// drawCollectionType draws a type length accepts.
func (g *gen) drawCollectionType(depth int) model.Type {
	switch rapid.IntRange(0, 3).Draw(g.t, "collection") {
	case 0:
		return model.StringType
	case 1:
		return model.NewListType(g.drawType(depth))
	case 2:
		return model.NewMapType(g.drawType(depth))
	default:
		return g.drawObjectType(depth+1, nil)
	}
}

func (g *gen) stringForms(depth int) []func() expr {
	str := func(src string) expr { return expr{src: src, typ: model.StringType} }
	operand := func() string { return g.expr(model.StringType, depth-1).src }
	nullary := func(name string) func() expr { return func() expr { return str(name + "()") } }
	return []func() expr{
		func() expr { return g.template(depth) },
		func() expr {
			return str(fmt.Sprintf("join(%s, %s)", operand(), g.expr(model.NewListType(model.StringType), depth-1).src))
		},
		func() expr { return str(fmt.Sprintf("toJSON(%s)", g.expr(g.drawType(2), depth-1).src)) },
		func() expr { return str(fmt.Sprintf("toBase64(%s)", operand())) },
		func() expr { return str(fmt.Sprintf("fromBase64(toBase64(%s))", operand())) },
		func() expr { return str(fmt.Sprintf("sha1(%s)", operand())) },
		nullary("cwd"),
		nullary("rootDirectory"),
		nullary("project"),
		nullary("stack"),
		nullary("organization"),
	}
}

// template draws a quoted string that interpolates scalar expressions.
func (g *gen) template(depth int) expr {
	var b strings.Builder
	b.WriteByte('"')
	for range rapid.IntRange(1, 3).Draw(g.t, "parts") {
		text := rapid.String().Draw(g.t, "text")
		// A "$" right before "${" would read as the "$${" escape, so a trailing run of "$"
		// is interpolated as a string literal instead.
		run := len(text) - len(strings.TrimRight(text, "$"))
		b.WriteString(quoteBody(text[:len(text)-run]))
		if run > 0 {
			b.WriteString("${" + Quote(text[len(text)-run:]) + "}")
		}
		b.WriteString("${" + g.expr(rapid.SampledFrom(scalarTypes).Draw(g.t, "scalar"), depth-1).src + "}")
	}
	b.WriteString(quoteBody(rapid.String().Draw(g.t, "text")))
	b.WriteByte('"')
	return expr{src: b.String(), typ: model.StringType}
}

func (g *gen) listForms(typ *model.ListType, depth int) []func() expr {
	elem := typ.ElementType
	forms := []func() expr{
		func() expr { return g.listFor(typ, depth) },
		func() expr {
			m := g.expr(model.NewMapType(elem), depth-1)
			e := g.fresh("e")
			return expr{src: fmt.Sprintf("[for %s in entries(%s) : %s.value]", e, m.src, e), typ: typ, minLen: len(m.keys)}
		},
		func() expr {
			key := identifier.Draw(g.t, "key")
			l := g.expr(model.NewListType(g.drawObjectType(depth-1, map[string]model.Type{key: elem})), depth-1)
			// A traversal that follows a splat applies to each item, so the splat is parenthesized.
			return expr{src: fmt.Sprintf("(%s[*].%s)", l.src, key), typ: typ, minLen: l.minLen}
		},
	}
	if elem.Equals(model.StringType) {
		// split of an empty string by an empty separator is empty, so the length has no lower bound.
		forms = append(forms, func() expr {
			return expr{src: fmt.Sprintf("split(%s, %s)", g.expr(model.StringType, depth-1).src,
				g.expr(model.StringType, depth-1).src), typ: typ}
		})
	}
	if elem.Equals(model.NumberType) {
		forms = append(forms, func() expr {
			from, to := rapid.IntRange(-3, 3).Draw(g.t, "from"), rapid.IntRange(-3, 6).Draw(g.t, "to")
			if rapid.Bool().Draw(g.t, "fromZero") {
				return expr{src: fmt.Sprintf("range(%d)", to), typ: typ, minLen: max(to, 0)}
			}
			return expr{src: fmt.Sprintf("range(%d, %d)", from, to), typ: typ, minLen: max(to-from, 0)}
		})
	}
	if obj, ok := elem.(*model.ObjectType); ok && isEntryType(obj) {
		forms = append(forms, func() expr {
			m := g.expr(model.NewMapType(obj.Properties["value"]), depth-1)
			return expr{src: fmt.Sprintf("entries(%s)", m.src), typ: typ, minLen: len(m.keys)}
		})
	}
	return forms
}

func isEntryType(t *model.ObjectType) bool {
	return len(t.Properties) == 2 && t.Properties["key"] != nil && t.Properties["key"].Equals(model.StringType) &&
		t.Properties["value"] != nil
}

// iteration draws the collection and loop variables of a for expression. The key binding
// is empty when the loop declares no key variable.
func (g *gen) iteration(depth int) (header string, coll expr, key, value binding) {
	elem := g.drawType(1)
	if rapid.Bool().Draw(g.t, "overMap") {
		coll = g.expr(model.NewMapType(elem), depth-1)
		key = binding{name: g.fresh("k"), expr: expr{typ: model.StringType}}
		value = binding{name: g.fresh("w"), expr: expr{typ: elem}}
		return fmt.Sprintf("for %s, %s in %s", key.name, value.name, coll.src), coll, key, value
	}
	coll = g.expr(model.NewListType(elem), depth-1)
	key = binding{name: g.fresh("i"), expr: expr{typ: model.NumberType}}
	value = binding{name: g.fresh("x"), expr: expr{typ: elem}}
	if rapid.Bool().Draw(g.t, "indexed") {
		return fmt.Sprintf("for %s, %s in %s", key.name, value.name, coll.src), coll, key, value
	}
	return fmt.Sprintf("for %s in %s", value.name, coll.src), coll, binding{}, value
}

func (g *gen) listFor(typ *model.ListType, depth int) expr {
	header, coll, key, value := g.iteration(depth)
	body, cond := g.forBody(typ.ElementType, depth, key, value)
	minLen := max(coll.minLen, len(coll.keys))
	if cond != "" {
		minLen = 0
	}
	return expr{src: fmt.Sprintf("[%s : %s%s]", header, body.src, cond), typ: typ, minLen: minLen}
}

// forBody draws the value and optional condition of a for expression with the loop
// variables in scope.
func (g *gen) forBody(typ model.Type, depth int, key, value binding) (body expr, cond string) {
	inScope := func(f func() expr) expr {
		if key.name == "" {
			return g.with(value.name, value.expr.ref(value.name), f)
		}
		return g.with(key.name, key.expr.ref(key.name), func() expr {
			return g.with(value.name, value.expr.ref(value.name), f)
		})
	}
	body = inScope(func() expr { return g.expr(typ, depth-1) })
	if rapid.Bool().Draw(g.t, "filtered") {
		cond = " if " + inScope(func() expr { return g.expr(model.BoolType, depth-1) }).src
	}
	return body, cond
}

func (g *gen) mapForms(typ *model.MapType, depth int) []func() expr {
	return []func() expr{func() expr {
		header, coll, key, value := g.iteration(depth)
		if key.name == "" {
			// A list without an index variable has no unique key to build; iterate with one.
			key = binding{name: g.fresh("i"), expr: expr{typ: model.NumberType}}
			header = fmt.Sprintf("for %s, %s in %s", key.name, value.name, coll.src)
		}
		body, cond := g.forBody(typ.ElementType, depth, key, value)
		keyExpr, keys := key.name, coll.keys
		if key.expr.typ.Equals(model.NumberType) {
			keyExpr = fmt.Sprintf("\"k${%s}\"", key.name)
			keys = nil
			for i := range coll.minLen {
				keys = append(keys, fmt.Sprintf("k%d", i))
			}
		}
		if cond != "" {
			keys = nil
		}
		return expr{src: fmt.Sprintf("{%s : %s => %s%s}", header, keyExpr, body.src, cond), typ: typ, keys: keys}
	}}
}

// Quote renders s as a PCL string literal that evaluates to s.
func Quote(s string) string {
	return `"` + quoteBody(s) + `"`
}

// quoteBody escapes s for the inside of a PCL quoted string.
func quoteBody(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r == '"' || r == '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case (r == '$' || r == '%') && i+1 < len(runes) && runes[i+1] == '{':
			// A run of k sigils before "{" must be emitted as k+1 sigils: the scanner reads
			// the last two as the escape and the rest as literal characters.
			b.WriteRune(r)
			b.WriteRune(r)
		case !unicode.IsPrint(r):
			if r > 0xFFFF {
				fmt.Fprintf(&b, `\U%08x`, r)
			} else {
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
