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
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Placeholder mints placeholders inside [Recursive]. Each call returns a new placeholder that stands for the body at
// the same position in the slice that the build function returns.
type Placeholder = func() Type

// build identifies one Recursive call.
type build struct {
	n    int
	done bool
}

// Recursive builds mutually recursive types. Each call to self mints one placeholder; the i-th call stands for the
// i-th body that build returns, and for result[i]. Placeholders are valid only inside build.
//
// A body must put every placeholder under a list, set, map, tuple, or object. A body that is a placeholder, or a
// union or eventual of one, with no such constructor in between has no unfolding, and Recursive panics.
func Recursive(fn func(self Placeholder) []Type) []Type {
	b := &build{}
	self := func() Type {
		contract.Assertf(!b.done, "a placeholder cannot be minted after Recursive returns")
		b.n++
		return mk(node{kind: kindVar, num: uint64(b.n - 1), v: b})
	}
	bodies := fn(self)
	b.done = true
	contract.Assertf(len(bodies) == b.n, "Recursive minted %d placeholders but returned %d bodies", b.n, len(bodies))
	if !slices.ContainsFunc(bodies, func(t Type) bool { return t.flags()&fVar != 0 }) {
		return bodies // Every body is already a canonical closed type.
	}
	return canonicalize(bodies, modeRaw, b)
}

// stateOf returns the recursive type whose unfolding is n, if there is one. Only a state of the group of a direct
// child of n qualifies: every state of a group has a child in its own group, and the children of n are canonical, so
// a type bisimilar to n is a state whose unfolding has the same children as n.
func stateOf(n node) (Type, bool) {
	var groups []list
	add := func(t Type) {
		if r := t.raw(); r.kind == kindRec && !slices.Contains(groups, r.b) {
			groups = append(groups, r.b)
		}
	}
	add(n.a)
	for _, v := range n.b.values() {
		add(v)
	}
	for _, g := range groups {
		for i := range g.len() {
			s := mk(node{kind: kindRec, b: g, num: uint64(i)})
			h := s.head()
			if h.kind != n.kind || h.str != n.str || h.num != n.num || h.a != n.a {
				continue
			}
			if h.b == n.b || n.kind == KindUnion && slices.Equal(sortTypes(h.b.values()), n.b.values()) {
				return s, true
			}
		}
	}
	return None, false
}

const noHead = "a recursive type must put its self reference under a list, set, map, tuple, or object"

// edge is a child of an automaton state: another state, or a closed type when state is negative.
type edge struct {
	t     Type
	state int
}

// state is one constructor of an automaton. A kindVar state is an alias: it denotes kids[0].
type state struct {
	kind  Kind
	str   string
	num   uint64
	names []string
	kids  []edge
}

type automaton struct {
	states []state
}

// canonicalize returns the canonical form of each root. When b is not nil, the roots are the bodies of that
// Recursive call and placeholders refer to them by slot. m is the eventual mode of the roots.
func canonicalize(roots []Type, m mode, b *build) []Type {
	ex := explorer{build: b, memo: map[Type]int{}, groups: map[list]int{}}
	var rootEdges []edge
	if b != nil {
		// Each slot is an alias state whose target is its body, so that a body can refer to any slot before the
		// body of that slot has been explored.
		ex.aut.states = make([]state, len(roots))
		for i, body := range roots {
			ex.aut.states[i] = state{kind: kindVar, kids: []edge{ex.explore(body)}}
			rootEdges = append(rootEdges, edge{state: i})
		}
	} else {
		for _, root := range roots {
			rootEdges = append(rootEdges, ex.explore(root))
		}
	}

	v := variants{src: &ex.aut, idx: map[[2]int]int{}}
	for i, e := range rootEdges {
		rootEdges[i] = v.edge(e, m)
	}
	aut := v.out

	for {
		aut.normalize()
		for i, e := range rootEdges {
			rootEdges[i] = aut.resolveEdge(e)
		}
		var changed bool
		aut, rootEdges, changed = aut.minimize(rootEdges)
		if !changed {
			break
		}
	}

	em := newEmitter(&aut)
	out := make([]Type, len(rootEdges))
	for i, e := range rootEdges {
		if e.state >= 0 && em.index[e.state] < 0 {
			em.visit(e.state)
		}
		out[i] = em.typeOf(e)
	}
	return out
}

// explorer turns a term into automaton states. Only nodes that hold a placeholder or a recursive type become
// states; every other child is a closed edge.
type explorer struct {
	build  *build
	aut    automaton
	memo   map[Type]int
	groups map[list]int
}

func (e *explorer) alloc(s state) int {
	e.aut.states = append(e.aut.states, s)
	return len(e.aut.states) - 1
}

func (e *explorer) explore(t Type) edge {
	r := t.raw()
	if r.flags&fSpecial == 0 {
		return edge{t: t, state: -1}
	}
	if r.kind == kindVar {
		contract.Assertf(r.v != nil && r.v == e.build,
			"a placeholder was used outside the Recursive call that created it")
		return edge{state: int(r.num)}
	}
	if i, ok := e.memo[t]; ok {
		return edge{state: i}
	}
	if r.kind == kindRec {
		return edge{state: e.importGroup(r.b) + int(r.num)}
	}
	i := e.alloc(state{kind: r.kind, str: r.str, num: r.num})
	e.memo[t] = i
	kids, names := kidsOf(r, e.explore)
	e.aut.states[i].kids, e.aut.states[i].names = kids, names
	return edge{state: i}
}

// importGroup adds the states of a group and returns the index of its first state.
func (e *explorer) importGroup(g list) int {
	if base, ok := e.groups[g]; ok {
		return base
	}
	states := g.values()
	base := len(e.aut.states)
	e.groups[g] = base
	e.aut.states = append(e.aut.states, make([]state, len(states))...)
	for k, st := range states {
		r := st.raw()
		kids, names := kidsOf(r, func(t Type) edge {
			if v := t.raw(); v.kind == kindVar {
				return edge{state: base + int(v.num)}
			}
			return e.explore(t)
		})
		e.aut.states[base+k] = state{kind: r.kind, str: r.str, num: r.num, kids: kids, names: names}
	}
	return base
}

// kidsOf applies f to each child of a constructor node and returns the edges, with the property names of an
// object.
func kidsOf(r node, f func(Type) edge) ([]edge, []string) {
	switch r.kind {
	case KindList, KindSet, KindMap, KindOutput, KindPromise:
		return []edge{f(r.a)}, nil
	case KindTuple, KindUnion, KindObject:
		kids := make([]edge, 0, r.b.len())
		var names []string
		for name, v := range r.b.items() {
			kids = append(kids, f(v))
			if r.kind == KindObject {
				names = append(names, name)
			}
		}
		return kids, names
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic, KindConst, KindEnum, KindOpaque,
		kindVar, kindRec:
	}
	panic(fmt.Sprintf("kind %d has no children", r.kind))
}

// variants copies an automaton with one state per (state, mode) pair reachable from the roots, so that the element
// of an output holds no eventual and the element of a promise holds no promise.
type variants struct {
	src *automaton
	out automaton
	idx map[[2]int]int
}

func (v *variants) edge(e edge, m mode) edge {
	if e.state < 0 {
		return edge{t: resolve(e.t, m), state: -1}
	}
	return edge{state: v.get(e.state, m)}
}

func (v *variants) get(s int, m mode) int {
	key := [2]int{s, int(m)}
	if i, ok := v.idx[key]; ok {
		return i
	}
	i := len(v.out.states)
	v.out.states = append(v.out.states, state{})
	v.idx[key] = i

	src := v.src.states[s]
	st := state{kind: src.kind, str: src.str, num: src.num, names: src.names}
	kidMode := m
	if src.kind == KindOutput {
		kidMode = modeNoEventual
		if m == modeNoEventual {
			st.kind = kindVar
		}
	}
	if src.kind == KindPromise {
		kidMode = max(m, modeNoPromise)
		if m >= modeNoPromise {
			st.kind = kindVar
		}
	}
	st.kids = make([]edge, len(src.kids))
	for k, kid := range src.kids {
		st.kids[k] = v.edge(kid, kidMode)
	}
	v.out.states[i] = st
	return i
}

// resolveEdge follows alias states to the state or closed type they denote.
func (a *automaton) resolveEdge(e edge) edge {
	seen := map[int]bool{}
	for e.state >= 0 && a.states[e.state].kind == kindVar {
		contract.Assertf(!seen[e.state], noHead)
		seen[e.state] = true
		e = a.states[e.state].kids[0]
	}
	return e
}

// normalize bypasses alias states and puts unions in normal form: members that are unions are flattened,
// duplicates are removed, and a union of one member becomes an alias of that member.
func (a *automaton) normalize() {
	for changed := true; changed; {
		changed = false
		for i := range a.states {
			for k, kid := range a.states[i].kids {
				a.states[i].kids[k] = a.resolveEdge(kid)
			}
		}
		for i := range a.states {
			if a.states[i].kind != KindUnion {
				continue
			}
			kids := a.flattenUnion(i, map[int]bool{})
			contract.Assertf(len(kids) > 0, "a union state has no members")
			if len(kids) == 1 {
				a.states[i] = state{kind: kindVar, kids: kids}
				changed = true
			} else {
				a.states[i].kids = kids
			}
		}
	}
}

// flattenUnion returns the distinct members of union state i after flattening members that are unions.
func (a *automaton) flattenUnion(i int, seen map[int]bool) []edge {
	contract.Assertf(!seen[i], noHead)
	seen[i] = true
	defer delete(seen, i)
	var out []edge
	add := func(e edge) {
		if !slices.Contains(out, e) {
			out = append(out, e)
		}
	}
	for _, kid := range a.states[i].kids {
		kid = a.resolveEdge(kid)
		switch {
		case kid.state >= 0 && a.states[kid.state].kind == KindUnion:
			for _, e := range a.flattenUnion(kid.state, seen) {
				add(e)
			}
		case kid.state < 0 && kid.t.raw().kind == KindUnion:
			for _, m := range kid.t.raw().b.values() {
				add(edge{t: m, state: -1})
			}
		default:
			add(kid)
		}
	}
	return out
}

// minimize merges bisimilar states and drops states that no root reaches. The last result reports whether a union
// lost members in the merge: the quotient then needs another pass, since the smaller union may be bisimilar to
// another state, and a union of one member is an alias state.
func (a *automaton) minimize(roots []edge) (automaton, []edge, bool) {
	n := len(a.states)
	closedIDs := map[Type]int{}
	closedID := func(t Type) int {
		if id, ok := closedIDs[t]; ok {
			return id
		}
		closedIDs[t] = len(closedIDs)
		return closedIDs[t]
	}

	tr := trie{nodes: map[[2]int]int{}}
	strIDs := map[string]int{}
	strID := func(str string) int {
		if id, ok := strIDs[str]; ok {
			return id
		}
		strIDs[str] = len(strIDs)
		return strIDs[str]
	}

	class := make([]int, n)
	classes := 0
	// assign computes every signature against the current classes before it overwrites any of them.
	assign := func(signature func(int) int) {
		signatures := make([]int, n)
		for i := range a.states {
			signatures[i] = signature(i)
		}
		ids := map[int]int{}
		for i, s := range signatures {
			if _, ok := ids[s]; !ok {
				ids[s] = len(ids)
			}
			class[i] = ids[s]
		}
		classes = len(ids)
	}
	var scratch []int
	// The first signature is the state's own data and the layout of its children: which closed type sits at each
	// position, or that a state does. A union has no positions, so its signature holds the sorted closed members
	// and the number of state members.
	assign(func(i int) int {
		s := a.states[i]
		id := tr.walk(0, int(s.kind))
		id = tr.walk(id, strID(s.str))
		id = tr.walk(id, int(s.num))
		id = tr.walk(id, len(s.names))
		for _, name := range s.names {
			id = tr.walk(id, strID(name))
		}
		if s.kind != KindUnion {
			for _, kid := range s.kids {
				if kid.state >= 0 {
					id = tr.walk(id, 0)
				} else {
					id = tr.walk(id, 1+closedID(kid.t))
				}
			}
			return id
		}
		scratch = scratch[:0]
		states := 0
		for _, kid := range s.kids {
			if kid.state >= 0 {
				states++
			} else {
				scratch = append(scratch, closedID(kid.t))
			}
		}
		slices.Sort(scratch)
		id = tr.walk(id, len(scratch))
		for _, c := range scratch {
			id = tr.walk(id, c)
		}
		return tr.walk(id, states)
	})
	for {
		before := classes
		assign(func(i int) int {
			s := a.states[i]
			id := tr.walk(0, class[i])
			scratch = scratch[:0]
			for _, kid := range s.kids {
				if kid.state >= 0 {
					scratch = append(scratch, class[kid.state])
				}
			}
			if s.kind == KindUnion {
				slices.Sort(scratch)
			}
			for _, c := range scratch {
				id = tr.walk(id, c)
			}
			return id
		})
		if classes == before {
			break
		}
	}

	// Number the classes that the roots reach, in discovery order.
	index := make([]int, classes)
	for i := range index {
		index[i] = -1
	}
	representative := make([]int, 0, classes)
	var reach func(e edge)
	reach = func(e edge) {
		if e.state < 0 || index[class[e.state]] >= 0 {
			return
		}
		index[class[e.state]] = len(representative)
		representative = append(representative, e.state)
		for _, kid := range a.states[e.state].kids {
			reach(kid)
		}
	}
	for _, root := range roots {
		reach(root)
	}

	remap := func(e edge) edge {
		if e.state < 0 {
			return e
		}
		return edge{state: index[class[e.state]]}
	}
	changed := false
	out := automaton{states: make([]state, len(representative))}
	for i, rep := range representative {
		s := a.states[rep]
		kids := make([]edge, 0, len(s.kids))
		for _, kid := range s.kids {
			kid = remap(kid)
			if s.kind != KindUnion || !slices.Contains(kids, kid) {
				kids = append(kids, kid)
			}
		}
		if len(kids) < len(s.kids) {
			changed = true
		}
		if s.kind == KindUnion && len(kids) == 1 {
			out.states[i] = state{kind: kindVar, kids: kids}
			continue
		}
		out.states[i] = state{kind: s.kind, str: s.str, num: s.num, names: s.names, kids: kids}
	}
	for i, root := range roots {
		roots[i] = remap(root)
	}
	return out, roots, changed
}

// emitter builds canonical types from a minimal automaton, one strongly connected component at a time in reverse
// topological order, so that the children outside a component are canonical types before the component is built.
type emitter struct {
	a       *automaton
	emitted map[int]Type

	index, low []int
	onStack    []bool
	stack      []int
	counter    int
}

func newEmitter(a *automaton) *emitter {
	n := len(a.states)
	e := &emitter{a: a, emitted: map[int]Type{}, index: make([]int, n), low: make([]int, n), onStack: make([]bool, n)}
	for i := range e.index {
		e.index[i] = -1
	}
	return e
}

func (e *emitter) typeOf(k edge) Type {
	if k.state < 0 {
		return k.t
	}
	return e.emitted[k.state]
}

// visit runs Tarjan's algorithm from v and emits each component when it completes.
func (e *emitter) visit(v int) {
	e.counter++
	e.index[v], e.low[v] = e.counter, e.counter
	e.stack = append(e.stack, v)
	e.onStack[v] = true
	for _, kid := range e.a.states[v].kids {
		if kid.state < 0 {
			continue
		}
		if e.index[kid.state] < 0 {
			e.visit(kid.state)
			e.low[v] = min(e.low[v], e.low[kid.state])
		} else if e.onStack[kid.state] {
			e.low[v] = min(e.low[v], e.index[kid.state])
		}
	}
	if e.low[v] != e.index[v] {
		return
	}
	var members []int
	for {
		w := e.stack[len(e.stack)-1]
		e.stack = e.stack[:len(e.stack)-1]
		e.onStack[w] = false
		members = append(members, w)
		if w == v {
			break
		}
	}
	e.emit(members)
}

// emit builds the types of one component. A component of one state with no edge to itself is a plain node over
// canonical children. Any other component becomes a group, with its states in canonical order.
func (e *emitter) emit(members []int) {
	if len(members) == 1 {
		s := e.a.states[members[0]]
		if !slices.ContainsFunc(s.kids, func(k edge) bool { return k.state == members[0] }) {
			kids := make([]Type, len(s.kids))
			for i, kid := range s.kids {
				kids[i] = e.typeOf(kid)
			}
			e.emitted[members[0]] = construct(s, kids)
			return
		}
	}

	label := e.label(members)
	states := make([]Type, len(members))
	for _, m := range members {
		s := e.a.states[m]
		kids := make([]Type, len(s.kids))
		for i, kid := range s.kids {
			if l, in := label[kid.state]; kid.state >= 0 && in {
				kids[i] = mk(node{kind: kindVar, num: uint64(l)})
			} else {
				kids[i] = e.typeOf(kid)
			}
		}
		states[label[m]] = construct(s, kids)
	}
	group := cells(states, nil)
	for _, m := range members {
		e.emitted[m] = mk(node{kind: kindRec, b: group, num: uint64(label[m])})
	}
}

// construct interns the node for s over the given children. Union members are sorted by Compare.
func construct(s state, kids []Type) Type {
	n := node{kind: s.kind, str: s.str, num: s.num}
	switch s.kind {
	case KindList, KindSet, KindMap, KindOutput, KindPromise:
		n.a = kids[0]
	case KindTuple:
		n.b = cells(kids, nil)
	case KindObject:
		n.b = cells(kids, s.names)
	case KindUnion:
		n.b = cells(sortTypes(kids), nil)
	case KindNone, KindBool, KindInt, KindNumber, KindString, KindID, KindDynamic, KindConst, KindEnum, KindOpaque,
		kindVar, kindRec:
		panic(fmt.Sprintf("kind %d is not a state", s.kind))
	}
	return mk(n)
}

// label assigns each state of a component a canonical index. States are ordered by their own data and their
// children outside the component, and ties are broken by refinement on the ranks of their children inside the
// component until every state has its own rank. A minimal automaton has no two states that stay tied.
func (e *emitter) label(members []int) map[int]int {
	in := map[int]bool{}
	for _, m := range members {
		in[m] = true
	}
	type shape struct {
		state  int
		kind   Kind
		str    string
		num    uint64
		names  []string
		layout []int  // 0 for a child outside the component, 1 for one inside, in child order; a count for a union
		outer  []Type // children outside the component, in child order, sorted for a union
		inner  []int  // states of the children inside the component, in child order
	}
	shapes := make([]*shape, len(members))
	for i, m := range members {
		s := e.a.states[m]
		sh := &shape{state: m, kind: s.kind, str: s.str, num: s.num, names: s.names}
		for _, kid := range s.kids {
			if kid.state >= 0 && in[kid.state] {
				sh.layout = append(sh.layout, 1)
				sh.inner = append(sh.inner, kid.state)
			} else {
				sh.layout = append(sh.layout, 0)
				sh.outer = append(sh.outer, e.typeOf(kid))
			}
		}
		if s.kind == KindUnion {
			slices.SortFunc(sh.outer, Compare)
			sh.layout = []int{len(sh.inner)}
		}
		shapes[i] = sh
	}

	slices.SortFunc(shapes, func(x, y *shape) int {
		return cmp.Or(
			cmp.Compare(x.kind, y.kind),
			strings.Compare(x.str, y.str),
			cmp.Compare(x.num, y.num),
			slices.Compare(x.names, y.names),
			slices.Compare(x.layout, y.layout),
			slices.CompareFunc(x.outer, y.outer, Compare),
		)
	})
	rank := map[int]int{}
	assignRanks := func(equal func(x, y *shape) bool) int {
		distinct := 0
		for i, sh := range shapes {
			if i == 0 || !equal(shapes[i-1], sh) {
				distinct++
			}
			rank[sh.state] = distinct - 1
		}
		return distinct
	}
	distinct := assignRanks(func(x, y *shape) bool {
		return x.kind == y.kind && x.str == y.str && x.num == y.num && slices.Equal(x.names, y.names) &&
			slices.Equal(x.layout, y.layout) && slices.Equal(x.outer, y.outer)
	})

	for distinct < len(shapes) {
		// Refine on the ranks of the previous round only, so that a rank assigned in this round does not leak into
		// the keys of the states that follow it.
		previous := maps.Clone(rank)
		keys := map[int][]int{}
		for _, sh := range shapes {
			ranks := make([]int, len(sh.inner))
			for i, s := range sh.inner {
				ranks[i] = previous[s]
			}
			if sh.kind == KindUnion {
				slices.Sort(ranks)
			}
			keys[sh.state] = ranks
		}
		slices.SortStableFunc(shapes, func(x, y *shape) int {
			return cmp.Or(cmp.Compare(previous[x.state], previous[y.state]), slices.Compare(keys[x.state], keys[y.state]))
		})
		next := assignRanks(func(x, y *shape) bool {
			return previous[x.state] == previous[y.state] && slices.Equal(keys[x.state], keys[y.state])
		})
		contract.Assertf(next > distinct, "internal error: the states of a minimal automaton did not separate")
		distinct = next
	}
	return rank
}

// trie interns sequences of ints: two sequences have the same id exactly when they are equal. An id is a trie node,
// keyed by its parent and its last element; the root is 0.
type trie struct {
	nodes map[[2]int]int
}

func (t *trie) walk(id, x int) int {
	key := [2]int{id, x}
	if n, ok := t.nodes[key]; ok {
		return n
	}
	n := len(t.nodes) + 1
	t.nodes[key] = n
	return n
}

// groupHeight is one more than the greatest height of a group that the states of g refer to, so that a group is
// taller than every group it reaches.
func groupHeight(g list) int {
	h := 0
	for _, st := range g.values() {
		h = max(h, heightOf(st))
	}
	return h + 1
}

// heightOf is the height of the tallest group inside t, or 0 when t holds no recursive type.
func heightOf(t Type) int {
	r := t.raw()
	switch {
	case r.kind == kindRec:
		return t.info().height
	case r.flags&fRec == 0:
		return 0
	}
	h := heightOf(r.a)
	for _, v := range r.b.values() {
		h = max(h, heightOf(v))
	}
	return h
}
