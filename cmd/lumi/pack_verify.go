// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"errors"

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
			// Create a compiler object and perform the verification.
			if !verify(cmd, args) {
				return errors.New("verification failed")
			}
			return nil
		}),
	}

	return cmd
}

// verify creates a compiler, much like compile, but only performs binding and verification on it.  If verification
// succeeds, the return value is true; if verification fails, errors will have been output, and the return is false.
func verify(cmd *cobra.Command, args []string) bool {
	// Prepare the compiler info and, provided it succeeds, perform the verification.
	if comp, pkg := prepareCompiler(cmd, args); comp != nil {
		// Now perform the compilation and extract the heap snapshot.
		if pkg == nil {
			return comp.Verify()
		}
		return comp.VerifyPackage(pkg)
	}

	return false
}
