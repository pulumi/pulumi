// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag/colors"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/resource"
	"github.com/marapongo/mu/pkg/util/contract"
)

func newPlanCmd() *cobra.Command {
	var delete bool
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
			// Perform the compilation and, if non-nil is returned, create a plan and print it.
			if mugl := compile(cmd, args); mugl != nil {
				// TODO: fetch the old plan for purposes of diffing.
				rs, err := resource.NewSnapshot(mugl) // create a resource snapshot from the object graph.
				if err != nil {
					fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
					os.Exit(-1)
				}

				// Create a new context for the plan operations.
				ctx := resource.NewContext()

				var plan resource.Plan
				if delete {
					// Generate a plan for deleting the entire snapshot.
					plan = resource.NewDeletePlan(ctx, rs)
				} else {
					// Generate a plan for creating the resources from scratch.
					plan = resource.NewCreatePlan(ctx, rs)
				}

				// Finally just pretty-print out the plan.
				printPlan(plan)
			}
		},
	}

	cmd.PersistentFlags().BoolVar(
		&delete, "delete", false,
		"Create a plan for deleting an entire snapshot")

	return cmd
}

func printPlan(plan resource.Plan) {
	// Now walk the plan's steps and and pretty-print them out.
	step := plan.Steps()
	for step != nil {
		var b bytes.Buffer

		// Print this step information (resource and all its properties).
		printStep(&b, step, "")

		// Now go ahead and emit the output to the console, and move on to the next step in the plan.
		// TODO: it would be nice if, in the output, we showed the dependencies a la `git log --graph`.
		s := colors.Colorize(b.String())
		fmt.Printf(s)

		step = step.Next()
	}
}

func printStep(b *bytes.Buffer, step resource.Step, indent string) {
	// First print out the operation.
	switch step.Op() {
	case resource.OpCreate:
		b.WriteString(colors.Green)
		b.WriteString("+ ")
	case resource.OpDelete:
		b.WriteString(colors.Red)
		b.WriteString("- ")
	default:
		b.WriteString("  ")
	}

	// Next print the resource moniker, properties, etc.
	printResource(b, step.Resource(), indent)

	// Finally make sure to reset the color.
	b.WriteString(colors.Reset)
}

func printResource(b *bytes.Buffer, res resource.Resource, indent string) {
	// First print out the resource type (since it is easy on the eyes).
	b.WriteString(fmt.Sprintf("%s:\n", string(res.Type())))

	// Now print out the moniker and, if present, the ID, as "pseudo-properties".
	indent += "    "
	b.WriteString(fmt.Sprintf("%s[m=%s]\n", indent, string(res.Moniker())))
	if id := res.ID(); id != "" {
		b.WriteString(fmt.Sprintf("%s[id=%s]\n", indent, string(id)))
	}

	// Print all of the properties associated with this resource.
	printObject(b, res.Properties(), indent)
}

func printObject(b *bytes.Buffer, props resource.PropertyMap, indent string) {
	// Compute the maximum with of property keys so we can justify everything.
	keys := resource.StablePropertyKeys(props)
	maxkey := 0
	for _, k := range keys {
		if len(k) > maxkey {
			maxkey = len(k)
		}
	}

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s%-"+strconv.Itoa(maxkey)+"s: ", indent, k))
		printProperty(b, props[k], indent)
	}
}

func printProperty(b *bytes.Buffer, v resource.PropertyValue, indent string) {
	if v.IsBool() {
		b.WriteString(fmt.Sprintf("%t", v.BoolValue()))
	} else if v.IsNumber() {
		b.WriteString(fmt.Sprintf("%v", v.NumberValue()))
	} else if v.IsString() {
		b.WriteString(fmt.Sprintf("\"%s\"", v.StringValue()))
	} else if v.IsResource() {
		b.WriteString(fmt.Sprintf("-> *%s", v.ResourceValue()))
	} else if v.IsArray() {
		b.WriteString(fmt.Sprintf("[\n"))
		for i, elem := range v.ArrayValue() {
			prefix := fmt.Sprintf("%s    [%d]: ", indent, i)
			b.WriteString(prefix)
			printProperty(b, elem, fmt.Sprintf("%-"+strconv.Itoa(len(prefix))+"s", ""))
		}
		b.WriteString(fmt.Sprintf("%s]", indent))
	} else {
		contract.Assert(v.IsObject())
		b.WriteString("{\n")
		printObject(b, v.ObjectValue(), indent+"    ")
		b.WriteString(fmt.Sprintf("%s}", indent))
	}
	b.WriteString("\n")
}

func isPrintableProperty(prop *rt.Pointer) bool {
	_, isfunc := prop.Obj().Type().(*symbols.FunctionType)
	return !isfunc
}
