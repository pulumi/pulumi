// Copyright 2016 Marapongo, Inc. All rights reserved.

package graph

import (
	"errors"
)

// TopSort topologically sorts the graph, yielding an array of nodes that are in dependency order, using a simple
// DFS-based algorithm.  The graph must be acyclic, otherwise this function will return an error.
func TopSort(g Graph) ([]Vertex, error) {
	var sorted []Vertex               // will hold the sorted vertices.
	tempmark := make(map[Vertex]bool) // temporary marks to detect cycles.
	mark := make(map[Vertex]bool)     // marks that will avoid visiting the same node twice.

	// Now enumerate the roots, topologically sorting their dependencies.
	roots := g.Roots()
	for _, r := range roots {
		if err := topvisit(r, &sorted, tempmark, mark); err != nil {
			return sorted, err
		}
	}
	return sorted, nil
}

func topvisit(n Vertex, sorted *[]Vertex, tempmark map[Vertex]bool, mark map[Vertex]bool) error {
	if tempmark[n] {
		// This is not a DAG!  Stop sorting right away, and issue an error.
		// TODO: use a real error here; and also ideally give an error message that makes sense (w/ the full cycle).
		return errors.New("Graph is not a DAG")
	}
	if !mark[n] {
		tempmark[n] = true
		for _, m := range n.Outs() {
			if err := topvisit(m.To(), sorted, tempmark, mark); err != nil {
				return err
			}
		}
		mark[n] = true
		tempmark[n] = false
		*sorted = append(*sorted, n)
	}
	return nil
}
