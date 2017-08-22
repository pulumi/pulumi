// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/engine"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

func newPackEvalCmd() *cobra.Command {
	var configEnv string
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!dotOutput, "TODO[pulumi/pulumi-fabric#235]: DOT files not yet supported")
			return engine.PackEval(configEnv, args)
		}),
	}

	cmd.PersistentFlags().StringVar(
		&configEnv, "config-env", "",
		"Apply configuration from the specified environment before evaluating the package")
	cmd.PersistentFlags().BoolVar(
		&dotOutput, "dot", false,
		"Output the graph as a DOT digraph (graph description language)")

	return cmd
}
