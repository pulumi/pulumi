// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/tokens"
)

func newHuskInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <husk>",
		Short: "Create an empty husk with the given name, ready for deployments",
		Long: "Create an empty husk with the given name, ready for deployments\n" +
			"\n" +
			"This command creates an empty husk with the given name.  It has no resources, but\n" +
			"afterwards it can become the target of a deployment using the `deploy` command.",
		Run: func(cmd *cobra.Command, args []string) {
			// Read in the name of the husk to use.
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "fatal: missing required husk name\n")
				os.Exit(-1)
			}

			husk := tokens.QName(args[0])
			create(cmd, args[1:], husk)
		},
	}
}
