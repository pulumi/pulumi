// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.0.1" // TODO: a real auto-incrementing version number.

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Mu's version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Mu version %v\n", version)
		},
	}
}
