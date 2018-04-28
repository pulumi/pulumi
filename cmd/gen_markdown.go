// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// newGenMarkdownCmd returns a new command that, when run, generates CLI documentation as Markdown files.
// It is hidden by default since it's not commonly used outside of our own build processes.
func newGenMarkdownCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen-markdown <DIR>",
		Args:   cmdutil.ExactArgs(1),
		Short:  "Generate Pulumi CLI documentation as Markdown (one file per command)",
		Hidden: true,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return doc.GenMarkdownTree(root, args[0])
		}),
	}
}
