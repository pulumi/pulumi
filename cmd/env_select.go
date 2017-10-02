// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newEnvSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select [<env>]",
		Short: "Switch the current workspace to the given environment",
		Long: "Switch the current workspace to the given environment.  This allows you to use\n" +
			"other commands like `config`, `preview`, and `push` without needing to specify the\n" +
			"environment name each and every time.\n" +
			"\n" +
			"If no <env> argument is supplied, the current environment is printed.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read in the name of the environment to switch to.
			if len(args) == 0 {
				name, err := getCurrentEnv()
				if err != nil {
					return err
				}

				fmt.Printf("%v\n", name)
			}

			return setCurrentEnv(tokens.QName(args[0]), true)
		}),
	}
}
