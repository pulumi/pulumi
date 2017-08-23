// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
)

func newEnvLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all known environments",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return lumiEngine.ListEnvs()
		}),
	}
}
