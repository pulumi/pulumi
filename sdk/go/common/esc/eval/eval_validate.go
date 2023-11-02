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
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/pulumi/esc/ast"
	"github.com/pulumi/esc/internal/util"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/esc/syntax"
)

// jsonRepr returns the JSON string representation of the given value.
func jsonRepr(v any) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<error formatting constant: %v>", err)
	}
	return string(bytes)
}

// validationLoc tracks the location to blame for a validation failure.
//
// The subtle bit here is that we may be validating a value that is not defined by a literal (i.e. it is defined by
// an interpolation, symbol expression, or builtin). In the case that it _is_ defined by a literal, we want to traverse
// the literal and blame the specific property/index/etc. that causes a validation failure. In the case that it is _not_
// defined by a literal, we want to blame the defining expression, but include the relative path to the property that
// causes a validation failure.
type validationLoc struct {
	x      *expr  // the expression that defines the value
	path   string // the relative path to the value
	prefix bool   // true if errorf should include the path as a prefix in errors
}

// index returns the validationLoc associated with the given index. If the location's expression is an array literal
// and the index is in range, then the returned location will refer to the array element at the given index. Otherwise,
// the returned location will refer to the original expression, but will include an appropriate path prefix.
func (l validationLoc) index(i int) validationLoc {
	list, isLiteral := l.x.repr.(*arrayExpr)
	if isLiteral && i < len(list.elements) {
		return validationLoc{
			x:    list.elements[i],
			path: fmt.Sprintf("[%v]", i),
		}
	}
	return validationLoc{
		x:      l.x,
		path:   fmt.Sprintf("%v[%v]", l.path, i),
		prefix: true,
	}
}

// property returns the validationLoc associated with the given property. If the location's expression is an object
// literal and the property exists, then the returned location will refer to the property with the given key. Otherwise,
// the returned location will refer to the original expression, but will include an appropriate path prefix.
//
// Note that this _intentionally does not_ traverse the object's base value. This keeps validation errors local to the
// value being validated. This should not be an issue in practice, in any case: validation errors currently only occur
// when values are passed to builtins. If a value passed to a builtin is a literal, then it has no base value.
// Otherwise, it must not be a literal and we won't be propagating validation locations anyway.
func (l validationLoc) property(k string) validationLoc {
	if obj, isLiteral := l.x.repr.(*objectExpr); isLiteral {
		if v, ok := obj.properties[k]; ok {
			return validationLoc{
				x:    v,
				path: util.JoinKey("", k),
			}
		}
	}
	return validationLoc{
		x:      l.x,
		path:   util.JoinKey(l.path, k),
		prefix: true,
	}
}

type validator struct {
	diags syntax.Diagnostics
}

// errorf issues a validation error at the given location.
func (e *validator) errorf(loc validationLoc, format string, args ...any) bool {
	if loc.prefix {
		format = fmt.Sprintf("%s: %s", loc.path, format)
	}
	diag := ast.ExprError(loc.x.repr.syntax(), fmt.Sprintf(format, args...))
	e.diags.Extend(diag)
	return false
}

// constError issues an error associated with an invalid value where a constant is expected.
func (e *validator) constError(loc validationLoc, expected any) bool {
	return e.errorf(loc, "expected %v", jsonRepr(expected))
}

// enumError issues an error associated with an invalid value where an enum is expected.
func (e *validator) enumError(loc validationLoc, expected []any) bool {
	if len(expected) == 1 {
		return e.constError(loc, expected[0])
	}
	return e.errorf(loc, "expected one of %v", jsonRepr(expected))
}

// typeError issues an error associated with an invalid type.
func (e *validator) typeError(loc validationLoc, expected, got string) bool {
	return e.errorf(loc, "expected %s, got %s", expected, got)
}

// isAny returns true if a schema is the Always schema.
func (e *validator) isAny(s *schema.Schema) bool {
	return s == nil || s.Always
}

// equalsConst returns true if the JSON value of v is equal to c.
func (e *validator) equalsConst(v *value, c any) bool {
	switch c := c.(type) {
	case nil:
		return v.repr == nil
	case bool:
		return v.repr == c
	case json.Number:
		return v.repr == c
	case string:
		return v.repr == c
	case []any:
		a, ok := v.repr.([]*value)
		if !ok || len(a) != len(c) {
			return false
		}
		for i, c := range c {
			if !e.equalsConst(a[i], c) {
				return false
			}
		}
		return true
	case map[string]any:
		m, ok := v.repr.(map[string]*value)
		if !ok || len(m) != len(c) {
			return false
		}
		for k, c := range c {
			v, ok := m[k]
			if !ok || !e.equalsConst(v, c) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// checkType validates the Type field of accept against the given actual type.
func (e *validator) checkType(actual string, accept *schema.Schema, loc validationLoc) bool {
	if accept.Type != "" && actual != accept.Type {
		return e.typeError(loc, accept.Type, actual)
	}
	return true
}

// validateSchemaType checks that accept validates x.
func (e *validator) validateSchemaType(x, accept *schema.Schema, loc validationLoc) bool {
	if e.isAny(accept) {
		return true
	}
	if accept.Never {
		return false
	}

	if e.isAny(x) {
		return true
	}
	if x.Never {
		return false
	}

	refOK := accept.GetRef() == nil || e.validateSchemaType(x, accept.GetRef(), loc)
	xRefOK := x.GetRef() == nil || e.validateSchemaType(x.GetRef(), accept, loc)
	xAnyOfOK := e.validateInputSchemaAnyOf(x, accept, loc)
	xOneOfOK := e.validateInputSchemaOneOf(x, accept, loc)
	typeOK := x.Type == "" || e.checkType(x.Type, accept, loc)
	anyOfOK := e.validateSchemaAnyOf(x, accept, loc)
	oneOfOK := e.validateSchemaOneOf(x, accept, loc)

	complexOK := true
	switch x.Type {
	case "array":
		complexOK = e.validateSchemaArray(x, accept, loc)
	case "object":
		complexOK = e.validateSchemaObject(x, accept, loc)
	}

	return refOK && xRefOK && xAnyOfOK && xOneOfOK && typeOK && anyOfOK && oneOfOK && complexOK
}

// validateInputSchemaAnyOf checks that accept validates the input schema x if x has an anyOf directive.
func (e *validator) validateInputSchemaAnyOf(x, accept *schema.Schema, loc validationLoc) bool {
	if len(x.AnyOf) == 0 {
		return true
	}

	var matched bool
	var allDiags syntax.Diagnostics
	for _, x := range x.AnyOf {
		var ee validator
		if ee.validateSchemaType(x, accept, loc) {
			matched = true
		}
		allDiags.Extend(ee.diags...)
	}
	if !matched {
		e.diags.Extend(allDiags...)
		e.errorf(loc, "at least one subschema must match")
		return false
	}
	return true
}

// validateInputSchemaOneOf checks that accept validates the input schema x if x has an oneOf directive.
func (e *validator) validateInputSchemaOneOf(x, accept *schema.Schema, loc validationLoc) bool {
	if len(x.OneOf) == 0 {
		return true
	}

	var matched bool
	var allDiags syntax.Diagnostics
	for _, x := range x.OneOf {
		var ee validator
		if ee.validateSchemaType(x, accept, loc) {
			matched = true
		}
		allDiags.Extend(ee.diags...)
	}
	if !matched {
		e.diags.Extend(allDiags...)
		e.errorf(loc, "at least one subschema must match")
		return false
	}
	return true
}

// validateSchemaAnyOf checks that the anyOf schema accept validates the input schema x.
func (e *validator) validateSchemaAnyOf(x, accept *schema.Schema, loc validationLoc) bool {
	if len(accept.AnyOf) == 0 {
		return true
	}

	var matched bool
	var allDiags syntax.Diagnostics
	for _, accept := range accept.AnyOf {
		var ee validator
		if ee.validateSchemaType(x, accept, loc) {
			matched = true
		}
		allDiags.Extend(ee.diags...)
	}
	if !matched {
		e.diags.Extend(allDiags...)
		e.errorf(loc, "at least one subschema must match")
		return false
	}
	return true
}

// validateSchemaOneOf checks that the oneOf schema accept validates the input schema x.
//
// Note that for the purposes of validation we treat oneOf like anyOf in order to deal with schemas that will resolve
// to a single value.
func (e *validator) validateSchemaOneOf(x, accept *schema.Schema, loc validationLoc) bool {
	if len(accept.OneOf) == 0 {
		return true
	}

	var matched bool
	var allDiags syntax.Diagnostics
	for _, accept := range accept.OneOf {
		var ee validator
		if ee.validateSchemaType(x, accept, loc) {
			matched = true
		}
		allDiags.Extend(ee.diags...)
	}
	if !matched {
		e.diags.Extend(allDiags...)
		e.errorf(loc, "at least one subschema must match")
		return false
	}
	return true
}

// validateSchemaArray checks that the array-typed schema accept validates the array-typed schema x. In order for accept
// to validate x:
//
// - All common PrefixItems in accept must validate the corresponding PrefixItem in x
// - If accept has more PrefixItems than x, then accept's extra PrefixItems must validate x's Items
// - If x has more PrefixItems than accept, then accept's Items must validate x's extra PrefixItems
// - If x's Items is not Never, then accept's Items must validate x's Items
func (e *validator) validateSchemaArray(x, accept *schema.Schema, loc validationLoc) bool {
	allOk := true

	i := 0
	xprefix, aprefix := x.PrefixItems, accept.PrefixItems
	for len(xprefix) > 0 && len(aprefix) > 0 {
		xp, ap := xprefix[0], aprefix[0]

		ok := e.validateSchemaType(xp, ap, loc.index(i))
		allOk = allOk && ok

		xprefix, aprefix, i = xprefix[1:], aprefix[1:], i+1
	}
	if len(xprefix) > 0 {
		for len(xprefix) > 0 {
			xp := xprefix[0]

			ok := e.validateSchemaType(xp, accept.Items, loc.index(i))
			allOk = allOk && ok

			xprefix, i = xprefix[1:], i+1
		}
	} else if len(aprefix) > 0 {
		for len(aprefix) > 0 {
			ap := aprefix[0]

			ok := e.validateSchemaType(x.Items, ap, loc.index(i))
			allOk = allOk && ok

			aprefix, i = aprefix[1:], i+1
		}
	}

	if x.Items != nil && !x.Items.Never {
		ok := e.validateSchemaType(x.Items, accept.Items, loc)
		allOk = allOk && ok
	}
	return allOk
}

// validateSchemaObject checks that the object-typed schema accept validates the object-typed schema x. In order for
// accept to validate x:
//
// - For each property P with schema S in x:
//   - If P is also in accept with schema T, then T must validate S
//   - If P is not in accept, then x's AdditionalProperties must validate S
//
// - If x does not have AdditionalProperties:
//   - All Required properties in accept must also be Required in x
//   - For each DependentRequired property P in accept, if P is Required in x, all of P's dependencies must also be
//     Required in x
//
// - If x _does_ have AdditionalProperties, then accept's AdditionalProperties must validate x's AdditionalProperties
func (e *validator) validateSchemaObject(x, accept *schema.Schema, loc validationLoc) bool {
	allOk := true

	for name, px := range x.Properties {
		if pa, ok := accept.Properties[name]; ok {
			ok := e.validateSchemaType(px, pa, loc.property(name))
			allOk = allOk && ok
		} else {
			ok := e.validateSchemaType(px, accept.AdditionalProperties, loc.property(name))
			allOk = allOk && ok
		}
	}

	if x.AdditionalProperties == nil {
		xreq := make(map[string]bool, len(x.Required))
		for _, name := range x.Required {
			xreq[name] = true
		}

		checkRequired := func(ra []string) bool {
			ok := true
			for _, name := range ra {
				if !xreq[name] {
					e.errorf(loc.property(name), "missing required property")
					ok = false
				}
			}
			return ok
		}
		ok := checkRequired(accept.Required)
		allOk = allOk && ok

		for name, req := range accept.DependentRequired {
			if xreq[name] {
				ok = checkRequired(req)
				allOk = allOk && ok
			}
		}
	} else if !x.AdditionalProperties.Never {
		ok := e.validateSchemaType(x.AdditionalProperties, accept.AdditionalProperties, loc)
		allOk = allOk && ok
	}

	return allOk
}

// validateValue checks that accept validates value.
func (e *validator) validateValue(v *value, accept *schema.Schema, loc validationLoc) bool {
	return e.validateElement(v, accept, validationLoc{x: v.def})
}

// validateElement checks that accept validates value.
func (e *validator) validateElement(v *value, accept *schema.Schema, loc validationLoc) bool {
	if err := accept.Compile(); err != nil {
		e.errorf(loc, "internal error: invalid schema: %w", err)
		return false
	}

	if e.isAny(accept) {
		return true
	}
	if accept.Never {
		return false
	}
	if v.unknown {
		return e.validateSchemaType(v.schema, accept, loc)
	}

	rok := accept.GetRef() == nil || e.validateElement(v, accept.GetRef(), loc)
	aok := e.validateAnyOf(v, accept, loc)
	ook := e.validateOneOf(v, accept, loc)
	cok := e.validateConst(v, accept, loc)
	eok := e.validateEnum(v, accept, loc)
	tok := e.validateType(v, accept, loc)
	return rok && aok && ook && cok && eok && tok
}

// validateAnyOf checks that the anyOf schema accept validates the input schema x.
func (e *validator) validateAnyOf(v *value, accept *schema.Schema, loc validationLoc) bool {
	if len(accept.AnyOf) == 0 {
		return true
	}

	var matched bool
	var allDiags syntax.Diagnostics
	for _, accept := range accept.AnyOf {
		var ee validator
		if ee.validateElement(v, accept, loc) {
			matched = true
		}
		allDiags.Extend(ee.diags...)
	}
	if !matched {
		e.diags.Extend(allDiags...)
		e.errorf(loc, "at least one subschema must match")
		return false
	}
	return true
}

// validateOneOf checks that the oneOf schema accept validates the input schema x.
func (e *validator) validateOneOf(v *value, accept *schema.Schema, loc validationLoc) bool {
	if len(accept.OneOf) == 0 {
		return true
	}

	var matched *validator
	var allDiags syntax.Diagnostics
	for _, accept := range accept.OneOf {
		var ee validator
		if ee.validateElement(v, accept, loc) {
			if matched != nil {
				e.errorf(loc, "exactly one subschema may match")
				return false
			}
			matched = &ee
		}
		allDiags.Extend(ee.diags...)
	}
	if matched == nil {
		e.diags.Extend(allDiags...)
		e.errorf(loc, "exactly one subschema must match")
		return false
	}
	return true
}

// validateConst checks that accept's Const validates value.
func (e *validator) validateConst(v *value, accept *schema.Schema, loc validationLoc) bool {
	if accept.Const == nil || e.equalsConst(v, accept.Const) {
		return true
	}
	return e.constError(loc, accept.Const)
}

// validateEnum checks that accept's Enum validates value.
func (e *validator) validateEnum(v *value, accept *schema.Schema, loc validationLoc) bool {
	if len(accept.Enum) == 0 {
		return true
	}
	for _, c := range accept.Enum {
		if e.equalsConst(v, c) {
			return true
		}
	}
	return e.enumError(loc, accept.Enum)
}

// validateType checks that accept's type-specific clauses validate value.
func (e *validator) validateType(v *value, accept *schema.Schema, loc validationLoc) bool {
	switch repr := v.repr.(type) {
	case nil:
		if !e.checkType("null", accept, loc) {
			return false
		}
		return true
	case bool:
		if !e.checkType("boolean", accept, loc) {
			return false
		}
		return true
	case json.Number:
		if !e.checkType("number", accept, loc) {
			return false
		}
		return e.validateNumber(repr, accept, loc)
	case string:
		if !e.checkType("string", accept, loc) {
			return false
		}
		return e.validateString(repr, accept, loc)
	case []*value:
		if !e.checkType("array", accept, loc) {
			return false
		}
		return e.validateArray(repr, accept, loc)
	case map[string]*value:
		if !e.checkType("object", accept, loc) {
			return false
		}
		return e.validateObject(v, accept, loc)
	default:
		panic(fmt.Errorf("illegal value of type %T", repr))
	}
}

// validateNumber checks that accept's number-specific clauses validate v.
func (e *validator) validateNumber(v json.Number, accept *schema.Schema, loc validationLoc) bool {
	n, _, err := big.ParseFloat(string(v), 10, 0, big.ToNearestEven)
	if err != nil {
		e.errorf(loc, "internal error: invalid number %q (%v)", v, err)
		return false
	}

	ok := true
	if m := accept.GetMultipleOf(); m != nil {
		var q big.Float
		q.Quo(n, m)
		if !q.IsInt() {
			e.errorf(loc, "expected a multiple of %v", accept.MultipleOf)
			ok = false
		}
	}

	if m := accept.GetMinimum(); m != nil && n.Cmp(m) < 0 {
		e.errorf(loc, "expected a number greater than or equal to %v", accept.Minimum)
		ok = false
	}
	if m := accept.GetExclusiveMinimum(); m != nil && n.Cmp(m) <= 0 {
		e.errorf(loc, "expected a number greater than %v", accept.ExclusiveMinimum)
		ok = false
	}
	if m := accept.GetMaximum(); m != nil && n.Cmp(m) > 0 {
		e.errorf(loc, "expected a number less than or equal to%v", accept.Maximum)
		ok = false
	}
	if m := accept.GetExclusiveMaximum(); m != nil && n.Cmp(m) >= 0 {
		e.errorf(loc, "expected a number less than %v", accept.ExclusiveMaximum)
		ok = false
	}
	return ok
}

// validateString checks that accept's string-specific clauses validate v.
func (e *validator) validateString(v string, accept *schema.Schema, loc validationLoc) bool {
	ok := true
	if m := accept.GetMinLength(); m != nil && uint(len(v)) < *m {
		e.errorf(loc, "expected a string of at least length %v", accept.MinLength)
		ok = false
	}
	if m := accept.GetMaxLength(); m != nil && uint(len(v)) > *m {
		e.errorf(loc, "expected a string of at most length %v", accept.MaxLength)
		ok = false
	}
	if p := accept.GetPattern(); p != nil && !p.MatchString(v) {
		e.errorf(loc, "string must match the pattern %q", p.String())
		ok = false
	}
	return ok
}

// validateString checks that accept's array-specific clauses validate v.
func (e *validator) validateArray(v []*value, accept *schema.Schema, loc validationLoc) bool {
	ok := true
	if m := accept.GetMinItems(); m != nil && uint(len(v)) < *m {
		e.errorf(loc, "expected an array with at least %v items", accept.MinItems)
		ok = false
	}
	if m := accept.GetMaxItems(); m != nil && uint(len(v)) > *m {
		e.errorf(loc, "expected an array with at most %v items", accept.MaxItems)
		ok = false
	}

	for i, v := range v {
		vloc := loc.index(i)
		if i < len(accept.PrefixItems) {
			if !e.validateValue(v, accept.PrefixItems[i], vloc) {
				ok = false
			}
		} else if !e.validateValue(v, accept.Items, vloc) {
			ok = false
		}
	}
	return ok
}

// validateString checks that accept's object-specific clauses validate v.
func (e *validator) validateObject(v *value, accept *schema.Schema, loc validationLoc) bool {
	keys := v.keys()

	ok := true
	if m := accept.GetMinProperties(); m != nil && uint(len(keys)) < *m {
		e.errorf(loc, "expected an object with at least %v properties", accept.MinProperties)
		ok = false
	}
	if m := accept.GetMaxProperties(); m != nil && uint(len(keys)) > *m {
		e.errorf(loc, "expected an object with at most %v properties", accept.MaxProperties)
		ok = false
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[k] = struct{}{}

		kv := v.property(nil, k)
		vloc := loc.property(k)

		if p, has := accept.Properties[k]; has {
			if !e.validateValue(kv, p, vloc) {
				ok = false
			}
		} else if !e.validateValue(kv, accept.AdditionalProperties, vloc) {
			ok = false
		}
	}

	var missing []string
	for _, k := range accept.Required {
		if _, has := keySet[k]; !has {
			missing = append(missing, k)
		}
	}
	for k, required := range accept.DependentRequired {
		if _, has := keySet[k]; has {
			for _, rk := range required {
				if _, has := keySet[rk]; !has {
					missing = append(missing, rk)
				}
			}
		}
	}
	if len(missing) != 0 {
		e.errorf(loc, "missing required properties: %s", strings.Join(missing, ", "))
		ok = false
	}

	return ok
}
