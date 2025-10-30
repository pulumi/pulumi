package dotconv

import dotconv "github.com/pulumi/pulumi/sdk/v3/pkg/graph/dotconv"

// Print prints a resource graph.
func Print(g graph.Graph, w io.Writer, dotFragment string) error {
	return dotconv.Print(g, w, dotFragment)
}

