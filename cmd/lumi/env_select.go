// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

func newEnvSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "select [<env>]",
		Aliases: []string{"checkout", "switch"},
		Short:   "Switch the current workspace to the given environment",
		Long: "Switch the current workspace to the given environment.  This allows you to use\n" +
			"other commands like `config`, `plan`, and `deploy` without needing to specify the\n" +
			"environment name each and every time.\n" +
			"\n" +
			"If no <env> argument is supplied, the current environment is printed.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to switch to.
			if len(args) == 0 {
				if name := getCurrentEnv(); name != "" {
					fmt.Println(name)
				}
			} else {
				name := tokens.QName(args[0])
				setCurrentEnv(name, true)
			}
			return nil
		}),
	}
}
