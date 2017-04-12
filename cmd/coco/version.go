// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.0.1" // TODO[pulumi/coconut#13]: a real auto-incrementing version number.

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Coconut's version number",
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Coconut version %v\n", version)
			return nil
		}),
	}
}
