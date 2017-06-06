// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dotconv convers a LumiGL graph into its DOT digraph equivalent.  This is useful for integration with various
// visualization tools, like Graphviz.  Please see http://www.graphviz.org/content/dot-language for a thorough
// specification of the DOT file format.
package dotconv

import (
	"bufio"
	"fmt"
	"io"
	"strconv"

	"github.com/pulumi/lumi/pkg/graph"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func Print(g graph.Graph, w io.Writer) error {
	// Allocate a new writer.  In general, we will ignore write errors throughout this function, for simplicity, opting
	// instead to return the result of flushing the buffer at the end, which is generally latching.
	b := bufio.NewWriter(w)

	// Print the graph header.
	if _, err := b.WriteString("strict digraph {\n"); err != nil {
		return err
	}

	// Initialize the frontier with unvisited graph vertices.
	queued := make(map[graph.Vertex]bool)
	frontier := make([]graph.Vertex, 0, len(g.Roots()))
	for _, root := range g.Roots() {
		to := root.To()
		queued[to] = true
		frontier = append(frontier, to)
	}

	// For now, we auto-generate IDs.
	// TODO[pulumi/lumi#76]: use the object URNs instead, once we have them.
	c := 0
	ids := make(map[graph.Vertex]string)
	getID := func(v graph.Vertex) string {
		if id, has := ids[v]; has {
			return id
		}
		id := "Resource" + strconv.Itoa(c)
		c++
		ids[v] = id
		return id
	}

	// Now, until the frontier is empty, emit entries into the stream.
	indent := "    "
	emitted := make(map[graph.Vertex]bool)
	for len(frontier) > 0 {
		// Dequeue the head of the frontier.
		v := frontier[0]
		frontier = frontier[1:]
		contract.Assert(!emitted[v])
		emitted[v] = true

		// Get and lazily allocate the ID for this vertex.
		id := getID(v)

		// Print this vertex; first its "label" (type) and then its direct dependencies.
		// IDEA: consider serializing properties on the node also.
		b.WriteString(fmt.Sprintf("%v%v", indent, id))
		if label := v.Label(); label != "" {
			b.WriteString(fmt.Sprintf(" [label=\"%v\"]", label))
		}
		b.WriteString(";\n")

		// Now print out all dependencies as "ID -> {A ... Z}".
		outs := v.Outs()
		if len(outs) > 0 {
			b.WriteString(fmt.Sprintf("%v%v -> {", indent, id))

			// Print the ID of each dependency and, for those we haven't seen, add them to the frontier.
			for i, out := range outs {
				to := out.To()

				if i > 0 {
					b.WriteString(" ")
				}
				b.WriteString(getID(to))

				if _, q := queued[to]; !q {
					queued[to] = true
					frontier = append(frontier, to)
				}
			}

			b.WriteString("}\n")
		}
	}

	// Finish the graph.
	b.WriteString("}\n")

	return b.Flush()
}
