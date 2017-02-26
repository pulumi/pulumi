// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/compiler"
	"github.com/pulumi/coconut/pkg/util/cmdutil"
	"github.com/pulumi/coconut/pkg/util/contract"
)

func newVerifyCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "verify [package]",
		Short: "Check that a Nut and its NutIL are correct",
		Long: "Check that a Nut and its NutIL are correct\n" +
			"\n" +
			"A Nut contains intermediate language (NutIL) that encodes symbols,\n" +
			"definitions, and executable code.  This NutIL must obey a set of specific rules\n" +
			"for it to be valid.  Otherwise, evaluating it will fail.\n" +
			"\n" +
			"The verify command thoroughly checks the NutIL against these rules, and issues\n" +
			"errors anywhere it doesn't obey them.  This is generally useful for tools developers\n" +
			"and can ensure that Nuts do not fail at runtime, when such invariants are checked.",
		Run: func(cmd *cobra.Command, args []string) {
			// In the case of an argument, load that specific package and new up a compiler based on its base path.
			// Otherwise, use the default workspace and package logic (which consults the current working directory).
			var success bool
			if len(args) == 0 {
				comp, err := compiler.Newwd(nil)
				if err != nil {
					contract.Failf("fatal: %v", err)
				}
				success = comp.Verify()
			} else {
				fn := args[0]
				if pkg := cmdutil.ReadPackageFromArg(fn); pkg != nil {
					var comp compiler.Compiler
					var err error
					if fn == "-" {
						comp, err = compiler.Newwd(nil)
					} else {
						comp, err = compiler.New(filepath.Dir(fn), nil)
					}
					if err != nil {
						contract.Failf("fatal: %v", err)
					}
					success = comp.VerifyPackage(pkg)
				}
			}

			if !success {
				fmt.Printf("fatal: Nut verification failed\n")
			}
		},
	}

	return cmd
}
