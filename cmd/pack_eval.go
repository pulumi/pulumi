// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/eval/heapstate"
	"github.com/pulumi/coconut/pkg/graph"
	"github.com/pulumi/coconut/pkg/graph/dotconv"
	"github.com/pulumi/coconut/pkg/tokens"
)

func newPackEvalCmd() *cobra.Command {
	var dotOutput bool
	var cmd = &cobra.Command{
		Use:   "eval [package] [-- [args]]",
		Short: "Evaluate a package and print the resulting objects",
		Long: "Evaluate a package and print the resulting objects\n" +
			"\n" +
			"A graph is a topologically sorted directed-acyclic-graph (DAG), representing a\n" +
			"collection of resources that may be used in a deployment operation like plan or apply.\n" +
			"This graph is produced by evaluating the contents of a blueprint package, and does not\n" +
			"actually perform any updates to the target environment.\n" +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a package elsewhere can be provided as the [package] argument.",
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			// Perform the compilation and, if non-nil is returned, output the graph.
			if result := compile(cmd, args, nil); result != nil {
				// Serialize that evaluation graph so that it's suitable for printing/serializing.
				g := result.Heap.G
				if dotOutput {
					// Convert the output to a DOT file.
					if err := dotconv.Print(g, os.Stdout); err != nil {
						return fmt.Errorf("failed to write DOT file to output: %v", err)
					}
				} else {
					// Just print a very basic, yet (hopefully) aesthetically pleasinge, ascii-ization of the graph.
					shown := make(map[graph.Vertex]bool)
					for _, root := range g.Objs() {
						printVertex(root.ToObj(), shown, "")
					}
				}
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(
		&dotOutput, "dot", false,
		"Output the graph as a DOT digraph (graph description language)")

	return cmd
}

// printVertex just pretty-prints a graph.  The output is not serializable, it's just for display purposes.
// TODO: option to print properties.
// TODO: full serializability, including a DOT file option.
func printVertex(v *heapstate.ObjectVertex, shown map[graph.Vertex]bool, indent string) {
	s := v.Obj().Type()
	if shown[v] {
		fmt.Printf("%v%v: <cycle...>\n", indent, s)
	} else {
		shown[v] = true // prevent cycles.
		fmt.Printf("%v%v:\n", indent, s)
		for _, out := range v.OutObjs() {
			printVertex(out.ToObj(), shown, indent+"    -> ")
		}
	}
}

// dashdashArgsToMap is a simple args parser that places incoming key/value pairs into a map.  These are then used
// during package compilation as inputs to the main entrypoint function.
// TODO: this is fairly rudimentary; we eventually want to support arrays, maps, and complex types.
func dashdashArgsToMap(args []string) core.Args {
	mapped := make(core.Args)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Eat - or -- at the start.
		if arg[0] == '-' {
			arg = arg[1:]
			if arg[0] == '-' {
				arg = arg[1:]
			}
		}

		// Now find a k=v, and split the k/v part.
		if eq := strings.IndexByte(arg, '='); eq != -1 {
			// For --k=v, simply store v underneath k's entry.
			mapped[tokens.Name(arg[:eq])] = arg[eq+1:]
		} else {
			if i+1 < len(args) && args[i+1][0] != '-' {
				// If the next arg doesn't start with '-' (i.e., another flag) use its value.
				mapped[tokens.Name(arg)] = args[i+1]
				i++
			} else if arg[0:3] == "no-" {
				// For --no-k style args, strip off the no- prefix and store false underneath k.
				mapped[tokens.Name(arg[3:])] = false
			} else {
				// For all other --k args, assume this is a boolean flag, and set the value of k to true.
				mapped[tokens.Name(arg)] = true
			}
		}
	}

	return mapped
}
