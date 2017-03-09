// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"errors"

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
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			// Create a compiler object and perform the verification.
			if !verify(cmd, args) {
				return errors.New("verification failed")
			}
			return nil
		}),
	}

	return cmd
}
