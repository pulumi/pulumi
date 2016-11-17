// Copyright 2016 Marapongo, Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"

	"github.com/marapongo/mu/cmd"
)

func main() {
	// Ensure the glog library has been initialized, including calling flag.Parse beforehand.
	flag.Parse()
	glog.Info("Mu CLI is running")
	defer glog.Flush()

	if err := cmd.NewMuCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}
