// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

var version = "<unknown>" // Our Makefiles override this by pasing -X main.version to the linker

func main() {
	if err := cmd.NewPulumiCmd(version).Execute(); err != nil {
		_, err = fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		contract.IgnoreError(err)
		os.Exit(-1)
	}
}
