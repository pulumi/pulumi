// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/spf13/cobra"
)

const version = "0.0.1" // TODO[pulumi/pulumi-fabric#13]: a real auto-incrementing version number.

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Lumi's version number",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Lumi version %v\n", version)
			return nil
		}),
	}
}
