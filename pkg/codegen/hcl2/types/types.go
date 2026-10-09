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

// Package types holds an immutable, interned representation of PCL types.
//
// Every constructor returns a type in normal form and interns it, so two types are equal under == exactly when
// they are structurally equal, including recursive types, which compare equal to their unfoldings. A Type is
// comparable and may be used as a map key.
package types

import (
	"fmt"
	"iter"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Type is a PCL type. The zero Type is None.
type Type struct {
	p *entry
}

// entry is the interned object behind a Type. Its identity is its node; the other fields are derived from the node
// and live exactly as long as it does.
type entry struct {
	node
	// rec holds the unfolding and height of a recursive type once they have been computed.
	rec atomic.Pointer[recInfo]
}

// recInfo is the derived data of a recursive type.
type recInfo struct {
	head   node
	height int
}

// node is the interned payload of a Type.
//
// A recursive type is a kindRec node: b lists the states of its group, one interned node per state, and num is the
// index of the state this type denotes. Inside a state, a child that refers to another state of the group is a
// kindVar node with a nil v and the state index in num. A placeholder from Recursive is a kindVar node whose v is
// the build that minted it.
type node struct {
	kind Kind
	// str holds a constant string or an enum token.
	str string
	// num holds a constant bool, the bits of a constant number, the state index of a kindRec node, or the slot of a
	// kindVar node.
	num uint64
	// a holds the element type of a collection or eventual, or the base type of a constant or enum.
	a Type
	// b holds the tuple elements, union members, object properties, enum values, or the states of a group.
	b list
	// v is the Recursive call that minted a placeholder.
	v *build
	// flags records which special nodes occur in the type.
	flags uint8
}

const (
	// fVar marks a type that contains a kindVar node.
	fVar uint8 = 1 << iota
	// fRec marks a type that contains a kindRec node.
	fRec
	// fOutput marks a type that contains an output at any depth.
	fOutput

	// fSpecial marks a type that is not a plain closed tree: it holds a placeholder or a recursive type.
	fSpecial = fVar | fRec
)

// list is an interned cons list of types. The zero list is empty.
type list struct {
	p *cell
}

type cell struct {
	// name is the property name on an object cell.
	name string
	// n is the number of cells from this cell to the end of the list.
	n     uint64
	value Type
	next  list
	flags uint8
}

// Kind identifies the constructor of a Type.
type Kind uint8

const (
	KindNone Kind = iota
	KindBool
	KindInt
	KindNumber
	KindString
	KindID
	KindDynamic
	KindConst
	KindEnum
	KindList
	KindSet
	KindMap
	KindTuple
	KindObject
	KindUnion
	KindOutput
	KindPromise
	KindOpaque

	// kindVar is a placeholder or a reference to a state of a group. It never reaches callers.
	kindVar Kind = 254
	// kindRec is a recursive type. Callers see the kind of its unfolding.
	kindRec Kind = 255
)

var kindNames = [...]string{
	KindNone:    "none",
	KindBool:    "bool",
	KindInt:     "int",
	KindNumber:  "number",
	KindString:  "string",
	KindID:      "id",
	KindDynamic: "dynamic",
	KindConst:   "const",
	KindEnum:    "enum",
	KindList:    "list",
	KindSet:     "set",
	KindMap:     "map",
	KindTuple:   "tuple",
	KindObject:  "object",
	KindUnion:   "union",
	KindOutput:  "output",
	KindPromise: "promise",
	KindOpaque:  "opaque",
}

// kindIdents holds the Go identifier of each kind: the Kind constant is "Kind" + ident, and the scalar value or
// constructor function is ident.
var kindIdents = [...]string{
	KindNone:    "None",
	KindBool:    "Bool",
	KindInt:     "Int",
	KindNumber:  "Number",
	KindString:  "String",
	KindID:      "ID",
	KindDynamic: "Dynamic",
	KindConst:   "Const",
	KindEnum:    "Enum",
	KindList:    "List",
	KindSet:     "Set",
	KindMap:     "Map",
	KindTuple:   "Tuple",
	KindObject:  "Object",
	KindUnion:   "Union",
	KindOutput:  "Output",
	KindPromise: "Promise",
	KindOpaque:  "Opaque",
}

func (k Kind) String() string { return kindNames[k] }

// GoString returns the Go expression for k, such as "types.KindList".
func (k Kind) GoString() string { return "types.Kind" + kindIdents[k] }

// The scalar types.
var (
	None    = Type{}
	Bool    = mk(node{kind: KindBool})
	Int     = mk(node{kind: KindInt})
	Number  = mk(node{kind: KindNumber})
	String  = mk(node{kind: KindString})
	ID      = mk(node{kind: KindID})
	Dynamic = mk(node{kind: KindDynamic})
)

// mk interns n after computing its flags from its children.
func mk(n node) Type {
	switch n.kind {
	case kindVar:
		n.flags = fVar
	case kindRec:
		n.flags = fRec | n.b.flags()&fOutput
	case KindOutput:
		n.flags = fOutput | n.a.flags()
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic, KindConst, KindEnum, KindList,
		KindSet, KindMap, KindTuple, KindObject, KindUnion, KindPromise, KindOpaque:
		n.flags = n.a.flags() | n.b.flags()
	}
	return Type{nodeTable.intern(n)}
}

// ContainsOutputs reports whether an output occurs at any depth of t.
func ContainsOutputs(t Type) bool { return t.flags()&fOutput != 0 }

// finish interns a constructor node over canonical children. A node that holds a placeholder stays as built until
// the enclosing Recursive call canonicalizes it. A node that holds a recursive type is canonical unless it is the
// unfolding of a state of one of its children's groups, in which case that state is the canonical form.
func finish(n node) Type {
	t := mk(n)
	if t.flags()&fSpecial == fRec {
		if s, ok := stateOf(n); ok {
			return s
		}
	}
	return t
}

func (t Type) flags() uint8 {
	if t == None {
		return 0
	}
	return t.p.flags
}

// raw returns the node of t without unfolding a recursive type.
func (t Type) raw() node {
	if t == None {
		return node{kind: KindNone}
	}
	return t.p.node
}

// info returns the derived data of a recursive type, computing it on first use.
func (t Type) info() *recInfo {
	if i := t.p.rec.Load(); i != nil {
		return i
	}
	i := &recInfo{head: unfold(t.p.node), height: groupHeight(t.p.b)}
	t.p.rec.Store(i)
	return i
}

// head returns the node of t with a recursive type unfolded one level, so that its kind is a constructor and its
// children are types.
func (t Type) head() node {
	n := t.raw()
	switch n.kind {
	case kindVar:
		contract.Failf("a placeholder has no type outside the Recursive call that created it")
	case kindRec:
		return t.info().head
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic, KindConst, KindEnum, KindList,
		KindSet, KindMap, KindTuple, KindObject, KindUnion, KindOutput, KindPromise, KindOpaque:
	}
	return n
}

// unfold replaces every state reference in the state that n denotes with the recursive type of that state.
func unfold(n node) node {
	project := func(t Type) Type {
		if r := t.raw(); r.kind == kindVar {
			return mk(node{kind: kindRec, b: n.b, num: r.num})
		}
		return t
	}
	h := n.b.values()[n.num].raw()
	h.a = project(h.a)
	if h.b != (list{}) {
		names := make([]string, 0, h.b.len())
		values := make([]Type, 0, h.b.len())
		for name, v := range h.b.items() {
			names = append(names, name)
			values = append(values, project(v))
		}
		if h.kind != KindObject {
			names = nil
		}
		h.b = cells(values, names)
	}
	return h
}

// LiteralValues are the Go representations of constant values. The Go type fixes the base type of the constant:
// bool is Bool, int64 is Int, float64 is Number, and string is String.
type LiteralValues interface {
	bool | int64 | float64 | string
}

// Const returns the type of the single value v.
func Const[T LiteralValues](v T) Type {
	switch v := any(v).(type) {
	case bool:
		var bits uint64
		if v {
			bits = 1
		}
		return mk(node{kind: KindConst, a: Bool, num: bits})
	case int64:
		return mk(node{kind: KindConst, a: Int, num: uint64(v)})
	case float64:
		return mk(node{kind: KindConst, a: Number, num: math.Float64bits(v)})
	case string:
		return mk(node{kind: KindConst, a: String, str: v})
	}
	panic("unreachable")
}

// Enum returns the enum type with the given token and values. The values are stored sorted and without
// duplicates. Callers must give every enum with the same token the same values.
func Enum[T LiteralValues](token string, values ...T) Type {
	contract.Assertf(len(values) > 0, "an enum must have at least one value")
	members := make([]Type, len(values))
	for i, v := range values {
		members[i] = Const(v)
	}
	return mk(node{kind: KindEnum, str: token, a: members[0].ConstBase(), b: cells(sortTypes(members), nil)})
}

// Opaque returns the nominal type with the given name. An opaque type has no structure: it converts only to itself,
// and two opaque types are equal when their names are equal.
func Opaque(name string) Type {
	contract.Assertf(name != "", "an opaque type must have a name")
	return mk(node{kind: KindOpaque, str: name})
}

// List returns list(t).
func List(t Type) Type { return finish(node{kind: KindList, a: t}) }

// Set returns set(t).
func Set(t Type) Type { return finish(node{kind: KindSet, a: t}) }

// Map returns map(t).
func Map(t Type) Type { return finish(node{kind: KindMap, a: t}) }

// Tuple returns tuple(types...).
func Tuple(types ...Type) Type { return finish(node{kind: KindTuple, b: cells(types, nil)}) }

// Object returns the object type with the given properties.
func Object(properties map[string]Type) Type {
	names := slices.Sorted(maps.Keys(properties))
	values := make([]Type, len(names))
	for i, name := range names {
		values[i] = properties[name]
	}
	return finish(node{kind: KindObject, b: cells(values, names)})
}

// Union returns the union of types in normal form: union members are flattened, the members are sorted by Compare,
// duplicates are removed, an empty union is None, and a union of one member is that member.
func Union(types ...Type) Type {
	members := make([]Type, 0, len(types))
	var flags uint8
	for _, t := range types {
		flags |= t.flags()
		switch r := t.raw(); {
		case r.kind == kindVar:
			members = append(members, t)
		case r.kind == KindUnion:
			members = append(members, r.b.values()...)
		case r.kind == kindRec && t.Kind() == KindUnion:
			members = append(members, t.UnionValues()...)
		default:
			members = append(members, t)
		}
	}
	if flags&fVar == 0 {
		members = sortTypes(members)
	} else {
		// Placeholders have no order; the enclosing Recursive call sorts the members when it canonicalizes the union.
		members = dedup(members)
	}
	switch len(members) {
	case 0:
		return None
	case 1:
		return members[0]
	}
	return finish(node{kind: KindUnion, b: cells(members, nil)})
}

// Output returns output(t). The element type holds no output and no promise at any depth.
func Output(t Type) Type {
	if t.flags()&fVar == 0 {
		return mk(node{kind: KindOutput, a: resolve(t, modeNoEventual)})
	}
	return finish(node{kind: KindOutput, a: t})
}

// Promise returns promise(t). The element type holds no promise at any depth.
func Promise(t Type) Type {
	if t.flags()&fVar == 0 {
		return mk(node{kind: KindPromise, a: resolve(t, modeNoPromise)})
	}
	return finish(node{kind: KindPromise, a: t})
}

// ResolveOutputs removes every output and promise wrapper at any depth of t.
func ResolveOutputs(t Type) Type { return resolve(t, modeNoEventual) }

// ResolvePromises removes every promise wrapper at any depth of t.
func ResolvePromises(t Type) Type { return resolve(t, modeNoPromise) }

// mode says which eventual wrappers a type may hold.
type mode uint8

const (
	// modeRaw keeps every eventual.
	modeRaw mode = iota
	// modeNoPromise removes promises.
	modeNoPromise
	// modeNoEventual removes outputs and promises.
	modeNoEventual
)

// resolve removes the eventual wrappers that m forbids at any depth of t. A placeholder is a leaf; the enclosing
// Recursive call resolves through it.
func resolve(t Type, m mode) Type {
	if m == modeRaw {
		return t
	}
	n := t.raw()
	each := func(ts []Type) []Type {
		for i, t := range ts {
			ts[i] = resolve(t, m)
		}
		return ts
	}
	switch n.kind {
	case kindRec:
		return canonicalize([]Type{t}, m, nil)[0]
	case KindOutput:
		if m != modeNoEventual {
			return t
		}
		return resolve(n.a, m)
	case KindPromise:
		return resolve(n.a, m)
	case KindList:
		return List(resolve(n.a, m))
	case KindSet:
		return Set(resolve(n.a, m))
	case KindMap:
		return Map(resolve(n.a, m))
	case KindTuple:
		return Tuple(each(n.b.values())...)
	case KindUnion:
		return Union(each(n.b.values())...)
	case KindObject:
		properties := make(map[string]Type, n.b.len())
		for name, v := range n.b.items() {
			properties[name] = resolve(v, m)
		}
		return Object(properties)
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic, KindConst, KindEnum, KindOpaque,
		kindVar:
		return t
	}
	panic("unreachable")
}

// sortTypes sorts types by Compare and removes duplicates.
func sortTypes(types []Type) []Type {
	slices.SortFunc(types, Compare)
	return slices.Compact(types)
}

// dedup removes duplicates from types and keeps the first occurrence of each.
func dedup(types []Type) []Type {
	out := types[:0]
	for _, t := range types {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// cells builds the list of values. When names is not nil, names[i] is the name of values[i].
func cells(values []Type, names []string) list {
	var l list
	for i := len(values) - 1; i >= 0; i-- {
		c := cell{n: uint64(len(values) - i), value: values[i], next: l, flags: values[i].flags() | l.flags()}
		if names != nil {
			c.name = names[i]
		}
		l = list{cellTable.intern(c)}
	}
	return l
}

func (l list) flags() uint8 {
	if l == (list{}) {
		return 0
	}
	return l.p.flags
}

func (l list) len() int {
	if l == (list{}) {
		return 0
	}
	return int(l.p.n)
}

// items iterates over the names and values of l.
func (l list) items() iter.Seq2[string, Type] {
	return func(yield func(string, Type) bool) {
		for l != (list{}) {
			c := l.p
			if !yield(c.name, c.value) {
				return
			}
			l = c.next
		}
	}
}

func (l list) values() []Type {
	values := make([]Type, 0, l.len())
	for _, v := range l.items() {
		values = append(values, v)
	}
	return values
}

func (t Type) expect(method string, kinds ...Kind) node {
	n := t.head()
	contract.Assertf(slices.Contains(kinds, n.kind), "cannot call .%s() on kind %s", method, n.kind)
	return n
}

// Kind returns the constructor of t.
func (t Type) Kind() Kind { return t.head().kind }

// Element returns the element type of a list, set, map, output, or promise.
func (t Type) Element() Type {
	return t.expect("Element", KindList, KindSet, KindMap, KindOutput, KindPromise).a
}

// Len returns the number of elements of a tuple, union, or enum, or the number of properties of an object.
func (t Type) Len() int {
	return t.expect("Len", KindTuple, KindObject, KindUnion, KindEnum).b.len()
}

// TupleValues returns the element types of a tuple.
func (t Type) TupleValues() []Type {
	return t.expect("TupleValues", KindTuple).b.values()
}

// UnionValues returns the members of a union in normal-form order.
func (t Type) UnionValues() []Type {
	return t.expect("UnionValues", KindUnion).b.values()
}

// ObjectValues iterates over the properties of an object in name order.
func (t Type) ObjectValues() iter.Seq2[string, Type] {
	return t.expect("ObjectValues", KindObject).b.items()
}

// EnumToken returns the token of an enum.
func (t Type) EnumToken() string { return t.expect("EnumToken", KindEnum).str }

// EnumBase returns the base type of an enum.
func (t Type) EnumBase() Type { return t.expect("EnumBase", KindEnum).a }

// EnumValues returns the values of an enum as constant types, in Compare order.
func (t Type) EnumValues() []Type {
	return t.expect("EnumValues", KindEnum).b.values()
}

// OpaqueName returns the name of an opaque type.
func (t Type) OpaqueName() string { return t.expect("OpaqueName", KindOpaque).str }

// ConstBase returns the base type of a constant.
func (t Type) ConstBase() Type { return t.expect("ConstBase", KindConst).a }

func (t Type) constOf(method string, base Type) node {
	n := t.expect(method, KindConst)
	contract.Assertf(n.a == base, "cannot call .%s() on a constant of base %s", method, n.a)
	return n
}

// ConstBoolValue returns the value of a bool constant.
func (t Type) ConstBoolValue() bool { return t.constOf("ConstBoolValue", Bool).num != 0 }

// ConstIntValue returns the value of an int constant.
func (t Type) ConstIntValue() int64 { return int64(t.constOf("ConstIntValue", Int).num) }

// ConstNumberValue returns the value of a number constant.
func (t Type) ConstNumberValue() float64 {
	return math.Float64frombits(t.constOf("ConstNumberValue", Number).num)
}

// ConstStringValue returns the value of a string constant.
func (t Type) ConstStringValue() string { return t.constOf("ConstStringValue", String).str }

// String renders t in the notation of the PCL type specification. A recursive type renders as
// "rec(t0 = ..., t1 = ...).ti", where each ti is a state of its group.
func (t Type) String() string {
	n := t.raw()
	switch n.kind {
	case kindVar:
		if n.v != nil {
			return "placeholder"
		}
		return "t" + strconv.FormatUint(n.num, 10)
	case kindRec:
		states := make([]string, 0, n.b.len())
		for i, state := range n.b.values() {
			states = append(states, fmt.Sprintf("t%d = %s", i, state))
		}
		return fmt.Sprintf("rec(%s).t%d", strings.Join(states, ", "), n.num)
	case KindConst:
		return "const(" + t.constString() + ")"
	case KindEnum:
		return fmt.Sprintf("enum(%s, %s, {%s})", n.str, n.a, join(t.EnumValues(), Type.constString))
	case KindList, KindSet, KindMap, KindOutput, KindPromise:
		return fmt.Sprintf("%s(%s)", n.kind, n.a)
	case KindTuple, KindUnion:
		return fmt.Sprintf("%s(%s)", n.kind, join(n.b.values(), Type.String))
	case KindObject:
		properties := make([]string, 0, n.b.len())
		for name, v := range n.b.items() {
			properties = append(properties, name+" = "+v.String())
		}
		return "object({" + strings.Join(properties, ", ") + "})"
	case KindOpaque:
		return "opaque(" + n.str + ")"
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic:
		return n.kind.String()
	}
	panic("unreachable")
}

// GoString returns a Go expression that constructs t, such as "types.List(types.Number)".
func (t Type) GoString() string {
	n := t.raw()
	switch n.kind {
	case kindVar:
		if n.v != nil {
			return "placeholder"
		}
		return "t" + strconv.FormatUint(n.num, 10)
	case kindRec:
		names := make([]string, 0, n.b.len())
		mints := make([]string, 0, n.b.len())
		for i := range n.b.len() {
			names = append(names, "t"+strconv.Itoa(i))
			mints = append(mints, "self()")
		}
		return fmt.Sprintf(
			"types.Recursive(func(self types.Placeholder) []types.Type { %s := %s; return []types.Type{%s} })[%d]",
			strings.Join(names, ", "), strings.Join(mints, ", "), join(n.b.values(), Type.GoString), n.num)
	case KindConst:
		return "types.Const(" + t.constGoString() + ")"
	case KindEnum:
		return fmt.Sprintf("types.Enum(%s, %s)", strconv.Quote(n.str), join(t.EnumValues(), Type.constGoString))
	case KindList, KindSet, KindMap, KindOutput, KindPromise:
		return fmt.Sprintf("types.%s(%s)", kindIdents[n.kind], n.a.GoString())
	case KindTuple, KindUnion:
		return fmt.Sprintf("types.%s(%s)", kindIdents[n.kind], join(n.b.values(), Type.GoString))
	case KindObject:
		properties := make([]string, 0, n.b.len())
		for name, v := range n.b.items() {
			properties = append(properties, strconv.Quote(name)+": "+v.GoString())
		}
		return "types.Object(map[string]types.Type{" + strings.Join(properties, ", ") + "})"
	case KindOpaque:
		return "types.Opaque(" + strconv.Quote(n.str) + ")"
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic:
		return "types." + kindIdents[n.kind]
	}
	panic("unreachable")
}

// constGoString returns a Go expression for the value of a constant whose type is one of LiteralValues.
func (t Type) constGoString() string {
	n := t.raw()
	switch n.a {
	case Bool:
		return strconv.FormatBool(n.num != 0)
	case Int:
		return "int64(" + strconv.FormatInt(int64(n.num), 10) + ")"
	case Number:
		v := math.Float64frombits(n.num)
		switch {
		case math.IsNaN(v):
			return "math.NaN()"
		case math.IsInf(v, 1):
			return "math.Inf(1)"
		case math.IsInf(v, -1):
			return "math.Inf(-1)"
		case v == 0 && math.Signbit(v):
			return "math.Copysign(0, -1)"
		}
		return "float64(" + strconv.FormatFloat(v, 'g', -1, 64) + ")"
	}
	return strconv.Quote(n.str)
}

func (t Type) constString() string {
	n := t.raw()
	switch n.a {
	case Bool:
		return strconv.FormatBool(n.num != 0)
	case Int:
		return strconv.FormatInt(int64(n.num), 10)
	case Number:
		return strconv.FormatFloat(math.Float64frombits(n.num), 'g', -1, 64)
	}
	return strconv.Quote(n.str)
}

func join(types []Type, f func(Type) string) string {
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = f(t)
	}
	return strings.Join(parts, ", ")
}
