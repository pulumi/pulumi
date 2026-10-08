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

// Package unify computes the one type that every input converts to, following §5 and §12.2 of the PCL type
// specification in pkg/codegen/hcl2/model/README.md.
package unify

import (
	"iter"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
)

// Types unifies its inputs. The function is n-ary and symmetric; callers must not fold it over a list, since the
// n-ary result differs from a fold in some cases (§6).
func Types(ts ...types.Type) types.Type {
	return types.Recursive(func(self types.Placeholder) []types.Type {
		u := &unifier{self: self, inFlight: map[types.Type]types.Type{}}
		u.mint()
		u.bodies[0] = u.unify(ts)
		return u.bodies
	})[0]
}

// unifier runs one unification inside one Recursive build. Slot 0 is the result. Every other slot is a type under
// construction (§12.2): the merge of a list of input objects, or of input sequences, that was entered again while
// in flight. The specification names only objects, since its types recurse only through objects; here a cycle may
// pass through a list, set, or tuple as well, and the same placeholder rule applies.
type unifier struct {
	self   types.Placeholder
	bodies []types.Type
	// inFlight maps the key of a member list under construction to its placeholder.
	inFlight map[types.Type]types.Type
	// stack holds the mapping member lists in flight at U-MapOf, each with the number of placeholders minted when
	// it entered.
	stack []frame
}

type frame struct {
	key    types.Type
	minted int
}

// mint returns a new placeholder and reserves its body slot.
func (u *unifier) mint() (types.Type, int) {
	u.bodies = append(u.bodies, types.None)
	return u.self(), len(u.bodies) - 1
}

// construct returns the placeholder for the merge of members when that merge is in flight, and otherwise mints one,
// builds the merge with build, and files it as the placeholder's body.
func (u *unifier) construct(members []types.Type, build func() types.Type) types.Type {
	k := key(members)
	if p, ok := u.inFlight[k]; ok {
		return p
	}
	p, slot := u.mint()
	u.inFlight[k] = p
	u.bodies[slot] = build()
	return p
}

// key turns a member list into a comparable map key.
func key(members []types.Type) types.Type { return types.Tuple(members...) }

// members returns the member list of a union and [t] for every other type.
func members(t types.Type) []types.Type {
	if t.Kind() == types.KindUnion {
		return t.UnionValues()
	}
	return []types.Type{t}
}

// norm is the canonical member list of §3 for the members of the given types.
func norm(ts []types.Type) []types.Type {
	return members(types.Union(ts...))
}

func (u *unifier) unify(ts []types.Type) types.Type {
	if len(ts) == 0 {
		return types.None // U-Empty
	}
	if !slices.ContainsFunc(ts, func(t types.Type) bool { return t != ts[0] }) {
		return ts[0] // U-Eq
	}
	return u.merge(norm(ts)) // U-Flatten
}

// merge applies the rules of §5.2 to a canonical member list.
func (u *unifier) merge(all []types.Type) types.Type {
	var none []types.Type
	rest := make([]types.Type, 0, len(all))
	for _, t := range all {
		if t == types.None {
			none = []types.Type{types.None} // U-None
		} else {
			rest = append(rest, t)
		}
	}
	kinds := func(t types.Type, ks ...types.Kind) bool { return slices.Contains(ks, t.Kind()) }
	switch {
	case slices.Contains(rest, types.Dynamic): // U-Dynamic
		return types.Union(append(none, types.Dynamic)...)
	case slices.ContainsFunc(rest, func(t types.Type) bool { return kinds(t, types.KindOutput) }) ||
		slices.ContainsFunc(rest, func(t types.Type) bool { return kinds(t, types.KindPromise) }) &&
			slices.ContainsFunc(rest, types.ContainsOutputs): // U-Output
		return types.Union(append(none, types.Output(u.unify(strip(rest))))...)
	case slices.ContainsFunc(rest, func(t types.Type) bool { return kinds(t, types.KindPromise) }): // U-Promise
		return types.Union(append(none, types.Promise(u.unify(strip(rest))))...)
	}
	// U-Union
	var seqs, mapLike, scalars []types.Type
	for _, t := range rest {
		switch {
		case kinds(t, types.KindList, types.KindSet, types.KindTuple):
			seqs = append(seqs, t)
		case kinds(t, types.KindMap, types.KindObject):
			mapLike = append(mapLike, t)
		default:
			scalars = append(scalars, t)
		}
	}
	result := none
	result = append(result, u.seq(seqs)...)
	result = append(result, u.mapClass(mapLike)...)
	result = append(result, scalar(scalars)...)
	return types.Union(result...)
}

// strip removes one top-level output or promise wrapper from each member.
func strip(ts []types.Type) []types.Type {
	out := make([]types.Type, len(ts))
	for i, t := range ts {
		out[i] = t
		if t.Kind() == types.KindOutput || t.Kind() == types.KindPromise {
			out[i] = t.Element()
		}
	}
	return out
}

// seq merges the sequence members: lists, sets, and tuples.
func (u *unifier) seq(ss []types.Type) []types.Type {
	if len(ss) == 0 {
		return nil
	}
	if len(ss) == 1 { // U-Eq
		return ss
	}
	return []types.Type{u.construct(ss, func() types.Type {
		if sameLengthTuples(ss) { // U-Tuple
			elems := make([]types.Type, ss[0].Len())
			for i := range elems {
				column := make([]types.Type, len(ss))
				for j, s := range ss {
					column[j] = s.TupleValues()[i]
				}
				elems[i] = u.unify(column)
			}
			return types.Tuple(elems...)
		}
		var elems []types.Type
		for _, s := range ss {
			if s.Kind() == types.KindTuple {
				elems = append(elems, s.TupleValues()...)
			} else {
				elems = append(elems, s.Element())
			}
		}
		if !slices.ContainsFunc(ss, func(s types.Type) bool { return s.Kind() != types.KindSet }) { // U-Set
			return types.Set(u.unify(elems))
		}
		return types.List(u.unify(elems)) // U-List
	})}
}

func sameLengthTuples(ss []types.Type) bool {
	for _, s := range ss {
		if s.Kind() != types.KindTuple || s.Len() != ss[0].Len() {
			return false
		}
	}
	return true
}

// mapClass merges the map members: maps and objects.
func (u *unifier) mapClass(os []types.Type) []types.Type {
	if len(os) == 0 {
		return nil
	}
	if len(os) == 1 { // U-Eq
		return os
	}
	if !slices.ContainsFunc(os, func(o types.Type) bool { return o.Kind() != types.KindObject }) { // U-Object
		return []types.Type{u.construct(os, u.objects(os))}
	}

	// U-MapOf. A member list that is in flight again is a cycle through a map, which no type can name, so it merges
	// to its union; unless a placeholder was minted since it entered, in which case the recursion ends at that
	// placeholder and the list merges again (§12.2).
	k := key(os)
	for i := len(u.stack) - 1; i >= 0; i-- {
		if u.stack[i].key != k {
			continue
		}
		if u.stack[i].minted == len(u.bodies) {
			return []types.Type{types.Union(os...)}
		}
		break
	}
	var values []types.Type
	for _, o := range os {
		if o.Kind() == types.KindObject {
			for _, t := range o.ObjectValues() {
				values = append(values, t)
			}
		} else {
			values = append(values, o.Element())
		}
	}
	u.stack = append(u.stack, frame{key: k, minted: len(u.bodies)})
	element := u.unify(values)
	u.stack = u.stack[:len(u.stack)-1]
	return []types.Type{types.Map(element)}
}

// objects returns a function that builds U-Object: an object with the union of the property names, where each
// property unifies the members' properties of that name, and None stands for a property a member lacks. Every object
// lists its properties in name order, so one walk over all the lists visits the names in order.
func (u *unifier) objects(os []types.Type) func() types.Type {
	return func() types.Type {
		cs := make([]cursor, len(os))
		for j, o := range os {
			cs[j].next, cs[j].stop = iter.Pull2(o.ObjectValues())
			cs[j].advance()
		}
		defer func() {
			for _, c := range cs {
				c.stop()
			}
		}()
		properties := map[string]types.Type{}
		for {
			name, any := "", false
			for _, c := range cs {
				if c.ok && (!any || c.name < name) {
					name, any = c.name, true
				}
			}
			if !any {
				return types.Object(properties)
			}
			column := make([]types.Type, len(cs))
			for j := range cs {
				column[j] = types.None
				if cs[j].ok && cs[j].name == name {
					column[j] = cs[j].value
					cs[j].advance()
				}
			}
			properties[name] = u.unify(column)
		}
	}
}

// cursor is a position in the property list of one object.
type cursor struct {
	next  func() (string, types.Type, bool)
	stop  func()
	name  string
	value types.Type
	ok    bool
}

func (c *cursor) advance() { c.name, c.value, c.ok = c.next() }

// scalar keeps the maximal members of the scalar class under the order of §5.3.
func scalar(ss []types.Type) []types.Type {
	var out []types.Type
	for _, s := range ss {
		if !slices.ContainsFunc(ss, func(t types.Type) bool { return below(s, t) }) {
			out = append(out, s)
		}
	}
	return out
}

// below reports whether s' is above s: s converts safely to s', and either s' does not convert safely back or s'
// sorts first, which breaks the tie between string and id in favor of string.
func below(s, above types.Type) bool {
	return s != above && convert.To(above, s) == convert.Safe &&
		(convert.To(s, above) != convert.Safe || types.Compare(above, s) < 0)
}
