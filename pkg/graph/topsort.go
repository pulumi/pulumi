// Copyright 2016-2018, Pulumi Corporation.
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

package graph

import (
	"github.com/pkg/errors"
)

// Topsort topologically sorts the graph, yielding an array of nodes that are in dependency order, using a simple
// DFS-based algorithm.  The graph must be acyclic, otherwise this function will return an error.
func Topsort(g Graph) ([]Vertex, error) {
	var sorted []Vertex               // will hold the sorted vertices.
	visiting := make(map[Vertex]bool) // temporary entries to detect cycles.
	visited := make(map[Vertex]bool)  // entries to avoid visiting the same node twice.

	// Now enumerate the roots, topologically sorting their dependencies.
	roots := g.Roots()
	for _, r := range roots {
		if err := topvisit(r.To(), &sorted, visiting, visited); err != nil {
			return sorted, err
		}
	}
	return sorted, nil
}

func topvisit(n Vertex, sorted *[]Vertex, visiting map[Vertex]bool, visited map[Vertex]bool) error {
	if visiting[n] {
		// This is not a DAG!  Stop sorting right away, and issue an error.
		// IDEA: return diagnostic information about why this isn't a DAG (e.g., full cycle path).
		return errors.New("Graph is not a DAG")
	}
	if !visited[n] {
		visiting[n] = true
		for _, m := range n.Outs() {
			if err := topvisit(m.To(), sorted, visiting, visited); err != nil {
				return err
			}
		}
		visited[n] = true
		visiting[n] = false
		*sorted = append(*sorted, n)
	}
	return nil
}
