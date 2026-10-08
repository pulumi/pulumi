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

package types

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rec builds one recursive type from a body over its own placeholder.
func rec(body func(self Type) Type) Type {
	return Recursive(func(self Placeholder) []Type {
		s := self()
		return []Type{body(s)}
	})[0]
}

func TestRecursiveUnfolds(t *testing.T) {
	t.Parallel()

	list := rec(List)
	assert.Equal(t, KindList, list.Kind())
	assert.Equal(t, list, list.Element())
	assert.Equal(t, list, list.Element().Element().Element())
	assert.Equal(t, list, List(list))
	assert.Equal(t, list, List(List(list)))

	tree := rec(func(s Type) Type { return Object(map[string]Type{"value": String, "children": List(s)}) })
	assert.Equal(t, KindObject, tree.Kind())
	assert.Equal(t, map[string]Type{"value": String, "children": List(tree)}, maps.Collect(tree.ObjectValues()))
	assert.Equal(t, tree, Object(map[string]Type{"value": String, "children": List(tree)}))
}

func TestRecursiveIsCanonical(t *testing.T) {
	t.Parallel()

	assert.Equal(t, rec(List), rec(func(s Type) Type { return List(List(s)) }))
	assert.Equal(t, rec(List), rec(func(s Type) Type { return List(List(List(s))) }))
	assert.NotEqual(t, rec(List), rec(Set))
	assert.NotEqual(t, rec(List), rec(func(s Type) Type { return List(Union(s, None)) }))

	// A body that does not use its placeholder is the closed type it names.
	assert.Equal(t, List(String), rec(func(Type) Type { return List(String) }))

	// Two groups that denote the same pair of types are the same group.
	pair := func(a, b Type) []Type {
		return Recursive(func(self Placeholder) []Type {
			x, y := self(), self()
			return []Type{Map(Tuple(a, y)), Map(Tuple(b, x))}
		})
	}
	assert.Equal(t, pair(String, Int), pair(String, Int))
	assert.Equal(t, pair(String, Int)[0].Element().TupleValues()[1], pair(String, Int)[1])
	// With equal data the two states are bisimilar and merge into one.
	assert.Equal(t, pair(String, String)[0], pair(String, String)[1])
	assert.Equal(t, rec(func(s Type) Type { return Map(Tuple(String, s)) }), pair(String, String)[0])
}

func TestRecursiveMintOrderBinds(t *testing.T) {
	t.Parallel()

	ts := Recursive(func(self Placeholder) []Type {
		x, y := self(), self()
		return []Type{List(y), Object(map[string]Type{"a": x})}
	})
	require.Len(t, ts, 2)
	assert.Equal(t, KindList, ts[0].Kind())
	assert.Equal(t, ts[1], ts[0].Element())
	assert.Equal(t, KindObject, ts[1].Kind())
	assert.Equal(t, map[string]Type{"a": ts[0]}, maps.Collect(ts[1].ObjectValues()))

	// A body that is another placeholder is an alias of that placeholder's type.
	alias := Recursive(func(self Placeholder) []Type {
		x, y := self(), self()
		return []Type{y, List(x)}
	})
	assert.Equal(t, alias[0], alias[1])
	assert.Equal(t, rec(List), alias[0])
}

func TestRecursiveEventuals(t *testing.T) {
	t.Parallel()

	list := rec(List)
	assert.Equal(t, Output(list), Output(Output(list)))
	assert.Equal(t, list, Output(list).Element())
	assert.Equal(t, list, ResolveOutputs(list))

	// An output inside the cycle resolves what is under it, so list(output(list(output(...)))) is
	// list(output(list(list(...)))): the cycle does not pass through the output.
	outputs := rec(func(s Type) Type { return List(Output(s)) })
	assert.Equal(t, List(Output(list)), outputs)
	assert.Equal(t, list, outputs.Element().Element())
	assert.Equal(t, Output(list), Output(outputs))
	assert.Equal(t, list, ResolveOutputs(outputs))
	assert.Equal(t, outputs, ResolvePromises(outputs))

	promises := rec(func(s Type) Type { return List(Promise(s)) })
	assert.Equal(t, list, ResolvePromises(promises))
	assert.Equal(t, Promise(list), Promise(promises))
	assert.Equal(t, Output(list), Output(promises))

	// An output around the cycle resolves the cycle under it.
	assert.Equal(t, Output(list), rec(func(s Type) Type { return Output(List(s)) }))
}

func TestRecursiveUnions(t *testing.T) {
	t.Parallel()

	optional := rec(func(s Type) Type { return List(Union(s, None)) })
	assert.Equal(t, KindList, optional.Kind())
	assert.Equal(t, []Type{None, optional}, optional.Element().UnionValues())
	assert.Equal(t, optional, Union(optional, optional))
	assert.Equal(t, optional.Element(), Union(None, optional))

	// A union that collapses after its members merge is the member.
	collapsed := Recursive(func(self Placeholder) []Type {
		x, y := self(), self()
		return []Type{List(Union(x, y)), List(Union(y, x))}
	})
	assert.Equal(t, rec(List), collapsed[0])
	assert.Equal(t, collapsed[0], collapsed[1])
}

// TestRebuildIsState checks that a constructor over the children of a state gives that state back, including a
// union state whose members are other states of the group, and that a constructor over other children stays plain.
func TestRebuildIsState(t *testing.T) {
	t.Parallel()

	// lr has three states: t0 = list(t1), t1 = list(t2), t2 = union(t0, t1).
	lr := rec(func(s Type) Type { return List(Union(List(s), s)) })
	u := lr.Element()
	require.Equal(t, KindUnion, u.Kind())
	members := u.UnionValues()
	assert.Equal(t, lr, List(u))
	assert.Equal(t, u, Union(members[1], members[0]))
	assert.Equal(t, members[0], List(members[1]))
	assert.NotEqual(t, lr, rec(List))

	plain := List(Union(rec(List), Int))
	assert.Equal(t, KindList, plain.Kind())
	assert.Equal(t, []Type{Int, rec(List)}, plain.Element().UnionValues())
	assert.NotEqual(t, rec(List), List(Union(rec(List), rec(Set))))
}

func TestRecursivePanics(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { rec(func(s Type) Type { return s }) })
	assert.Panics(t, func() { rec(func(s Type) Type { return Union(s, Int) }) })
	assert.Panics(t, func() { rec(func(s Type) Type { return Output(s) }) })
	assert.Panics(t, func() { rec(func(s Type) Type { return Promise(Output(s)) }) })
	assert.Panics(t, func() {
		Recursive(func(self Placeholder) []Type {
			x, y := self(), self()
			return []Type{y, x}
		})
	})
	assert.Panics(t, func() { Recursive(func(self Placeholder) []Type { self(); return nil }) })
	assert.Panics(t, func() { Recursive(func(Placeholder) []Type { return []Type{String} }) })

	var leaked Type
	Recursive(func(self Placeholder) []Type {
		leaked = self()
		return []Type{List(leaked)}
	})
	assert.Panics(t, func() { leaked.Kind() })
	assert.Panics(t, func() {
		Recursive(func(self Placeholder) []Type {
			self()
			return []Type{List(leaked)}
		})
	})
	var mint Placeholder
	Recursive(func(self Placeholder) []Type {
		mint = self
		return []Type{List(self())}
	})
	assert.Panics(t, func() { mint() })
}

func TestRecursiveStrings(t *testing.T) {
	t.Parallel()

	list := rec(List)
	assert.Equal(t, "rec(t0 = list(t0)).t0", list.String())
	assert.Equal(t,
		"types.Recursive(func(self types.Placeholder) []types.Type { t0 := self(); return []types.Type{types.List(t0)} })[0]",
		list.GoString())

	pair := Recursive(func(self Placeholder) []Type {
		x, y := self(), self()
		return []Type{List(y), Object(map[string]Type{"a": x})}
	})
	assert.Equal(t, "rec(t0 = list(t1), t1 = object({a = t0})).t0", pair[0].String())
	assert.Equal(t, "rec(t0 = list(t1), t1 = object({a = t0})).t1", pair[1].String())
	assert.Equal(t, "tuple(rec(t0 = list(t0)).t0, int)", Tuple(list, Int).String())
}
