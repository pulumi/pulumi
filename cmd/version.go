// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/spf13/cobra"
)

const version = "0.6.1" // TODO[pulumi/pulumi#13]: a real auto-incrementing version number.

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Pulumi's version number",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Pulumi version %v\n", version)
			return nil
		}),
	}
}
