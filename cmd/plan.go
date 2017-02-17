// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/compiler/types/predef"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/graph"
)

func newPlanCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "plan [blueprint] [-- [args]]",
		Short: "Generate a deployment plan from a Mu blueprint",
		Long: "Generate a deployment plan from a Mu blueprint.\n" +
			"\n" +
			"A plan describes the overall graph and set of operations that will be performed\n" +
			"as part of a Mu deployment.  No actual resource creations, updates, or deletions\n" +
			"will take place.  This plan is as complete as possible without actually performing\n" +
			"the operations described in the plan (with the caveat that conditional execution\n" +
			"may obscure certain details, something that will be evident in plan's output).\n" +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a blueprint elsewhere can be provided as the [blueprint] argument.",
		Run: func(cmd *cobra.Command, args []string) {
			// Perform the compilation and, if non-nil is returned, output the plan.
			if mugl := compile(cmd, args); mugl != nil {
				printPlan(mugl)
			}
		},
	}

	return cmd
}

func printPlan(mugl graph.Graph) {
	// Sort the graph output so that it's a DAG.
	// TODO: consider pruning out all non-resources so there is less to sort.
	sorted, err := graph.TopSort(mugl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(-1)
	}

	// Now walk the elements and (for now), just print out which resources will be created.
	for _, vert := range sorted {
		o := vert.Obj()
		t := o.Type()
		if types.HasBaseName(t, predef.MuResourceClass) {
			// Print the resource type.
			fmt.Printf("+ %v:\n", t)

			// Print all of the properties associated with this resource.
			printProperties(o, "    ")
		}
	}
}

func printProperties(obj *rt.Object, indent string) {
	var keys []rt.PropertyKey
	props := obj.PropertyValues()

	// Compute the maximum with of property keys so we can justify everything.
	maxkey := 0
	for _, k := range rt.StablePropertyKeys(props) {
		if isPrintableProperty(props[k]) {
			keys = append(keys, k)
			if len(k) > maxkey {
				maxkey = len(k)
			}
		}
	}

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		fmt.Printf("%v%-"+strconv.Itoa(maxkey)+"s: ", indent, k)
		printProperty(props[k].Obj(), indent)
	}
}

func printProperty(obj *rt.Object, indent string) {
	switch obj.Type() {
	case types.Bool, types.Number, types.String:
		fmt.Printf("%v\n", obj)
	default:
		switch obj.Type().(type) {
		case *symbols.ArrayType:
			fmt.Printf("[\n")
			for i, elem := range *obj.ArrayValue() {
				fmt.Printf("%v    [%d]: ", indent, i)
				printProperty(elem.Obj(), fmt.Sprintf(indent+"        "))
			}
			fmt.Printf("%s]\n", indent)
		default:
			fmt.Printf("<%s> {\n", obj.Type())
			printProperties(obj, indent+"    ")
			fmt.Printf("%s}\n", indent)
		}
	}
}

func isPrintableProperty(prop *rt.Pointer) bool {
	_, isfunc := prop.Obj().Type().(*symbols.FunctionType)
	return !isfunc
}
