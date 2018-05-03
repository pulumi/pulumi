// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// newGenBashCompletionCmd returns a new command that, when run, generates a bash completion script for the CLI.
// It is hidden by default since it's not commonly used outside of our own build processes.
func newGenBashCompletionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen-bash-completion <FILE>",
		Args:   cmdutil.ExactArgs(1),
		Short:  "Generate a bash completion script for the Pulumi CLI",
		Hidden: true,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return root.GenBashCompletionFile(args[0])
		}),
	}
}
