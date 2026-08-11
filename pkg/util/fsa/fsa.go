package fsa

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

type FSA[Cursor any] struct {
	*fsa[Cursor]
}

func New[Cursor any]() FSA[Cursor] {
	return FSA[Cursor]{&fsa[Cursor]{
		cursors: map[nodeID]*cursor[Cursor]{},
		nodes:   map[nodeID]*node[Cursor]{},
		edges:   map[edgeID]ConditionFunc[Cursor]{},
	}}
}

// TODO[https://github.com/golang/go/issues/75757]: Should be a type alias
type NodeFunc[Cursor any] func(context.Context, FSA[Cursor], Edge, Cursor) error

type Node struct{ id nodeID }

// TODO[https://github.com/golang/go/issues/75757]: Should be a type alias
type ConditionFunc[Cursor any] func(ctx context.Context, fsa FSA[Cursor], from, to Node) (ConditionResult, error)

type ConditionResult struct{ kind int8 }

var (
	ConditionUnknown = ConditionResult{0 /* The zero value */} // Error
	ConditionPass    = ConditionResult{1}
	ConditionFail    = ConditionResult{2}
)

type Edge struct {
	id edgeID
}

func (fsa FSA[Cursor]) NewNode(f NodeFunc[Cursor]) Node {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	id := nodeID(fsa.idCounter)
	fsa.nodes[id] = &node[Cursor]{f, nil}
	return Node{id}
}

func (fsa FSA[Cursor]) NewEdge(f ConditionFunc[Cursor], from, to Node) Edge {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	id := edgeID(fsa.idCounter)
	fsa.edges[id] = f
	fsa.nodes[from.id].edges = append(fsa.nodes[from.id].edges,
		struct {
			e edgeID
			n nodeID
		}{id, to.id})
	return Edge{id}
}

// Place a cursor on n.
//
// The entry function for n is not called.
func (fsa FSA[Cursor]) NewCursor(c Cursor, n Node) {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	fsa.cursors[n.id] = &cursor[Cursor]{c, 0}
}

type Step[Cursor any] struct {
	Cursor Cursor                        // The cursor the step is for
	Do     func(context.Context, Cursor) // The function the user must call to complete it
}

// Iterate the FSA, supplying steps to call.
//
// Steps may be either a node or condition function
func (fsa FSA[Cursor]) Progress(ctx context.Context) error {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	toProgress := slices.Sorted(maps.Keys(fsa.cursors))

	fsa.generation++ // Invalidate previous generations

progressNode:
	for len(toProgress) > 0 {
		inProgress := toProgress[len(toProgress)-1]
		toProgress = toProgress[:len(toProgress)-1]
		c := fsa.nodes[inProgress]

		fmt.Printf("Progressing %#v\n", fsa.cursors[inProgress])

		if len(c.edges) == 0 {
			// The cursor is terminal, but not blocked
			continue progressNode
		}
		for _, e := range c.edges {
			var result ConditionResult
			var err error
			func() {
				fsa.m.Unlock()
				defer fsa.m.Lock()
				result, err = fsa.edges[e.e](ctx, fsa, Node{inProgress}, Node{e.n})
			}()
			if err != nil {
				return err
			}
			switch result {
			case ConditionFail:
				// Not this condition this time
			case ConditionUnknown:
				return errors.New("condition returned unknown")
			case ConditionPass:
				// We are now going to try to move down this path
				c := fsa.cursors[inProgress]
				func() {
					fsa.m.Unlock()
					defer fsa.m.Lock()
					err = fsa.nodes[e.n].f(ctx, fsa, Edge{e.e}, c.c)
				}()
				if err != nil {
					return err
				}
				// The walk didn't error, so we can
				delete(fsa.cursors, inProgress)
				fsa.cursors[e.n] = c
				toProgress = append(toProgress, e.n)
				continue progressNode
			}
		}

		// Mark this node as parked
		fsa.cursors[inProgress].parked = fsa.generation
	}
	return nil
}

// The subset of cursors that are parked
func (fsa FSA[Cursor]) Parked(yield func(Cursor, Node) bool) {
	fsa.cursorsInner(yield, true)
}

// The list of cursors and the node they are on
func (fsa FSA[Cursor]) Cursors(yield func(Cursor, Node) bool) {
	fsa.cursorsInner(yield, false)
}

func (fsa FSA[Cursor]) GetEdge(e Edge) (from, to Node) {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	for nID, n := range fsa.nodes {
		for _, pE := range n.edges {
			if pE.e == e.id {
				return Node{nID}, Node{pE.n}
			}
		}
	}
	panic("Edoge was not built from this graph")
}

func (fsa FSA[Cursor]) cursorsInner(yield func(Cursor, Node) bool, onlyParked bool) {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	for _, n := range slices.Sorted(maps.Keys(fsa.cursors)) {
		c, ok := fsa.cursors[n]
		if !ok || (onlyParked && (c.parked == 0 || c.parked < fsa.generation)) {
			continue
		}
		{
			fsa.m.Unlock()
			defer fsa.m.Lock()
			if !yield(c.c, Node{n}) {
				return
			}
		}
	}
}

// The core of the FSA
type fsa[Cursor any] struct {
	m sync.Mutex

	idCounter uint64

	generation uint64

	cursors map[nodeID]*cursor[Cursor]
	nodes   map[nodeID]*node[Cursor]
	edges   map[edgeID]ConditionFunc[Cursor]
}

type node[Cursor any] struct {
	f     NodeFunc[Cursor]
	edges []struct {
		e edgeID
		n nodeID
	}
}

type cursor[Cursor any] struct {
	c      Cursor
	parked uint64 // 0 means not parked, otherwise its the cycle where cursor was parked
}

type nodeID uint64 // A UUID for the node
type edgeID uint64 // A UUID for the edge
