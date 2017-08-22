// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi-fabric/pkg/engine"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/spf13/cobra"
)

func newPackVerifyCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "verify [package]",
		Short: "Check that a package's metadata and IL are correct",
		Long: "Check that a package's metadata and IL are correct\n" +
			"\n" +
			"A package contains intermediate language (IL) that encodes symbols, definitions,\n" +
			"and executable code.  This IL must obey a set of specific rules for it to be considered\n" +
			"legal and valid.  Otherwise, evaluating it will fail.\n" +
			"\n" +
			"The verify command thoroughly checks the package's IL against these rules, and issues\n" +
			"errors anywhere it doesn't obey them.  This is generally useful for tools developers\n" +
			"and can ensure that code does not fail at runtime, when such invariants are checked.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return engine.PackVerify(pkgargFromArgs(args))
		}),
	}

	return cmd
}
