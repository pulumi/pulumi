// Copyright 2016 Marapongo, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/marapongo/mu/cmd"
)

func main() {
	if err := cmd.NewMuCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}
