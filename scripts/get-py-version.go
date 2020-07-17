package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/buildutil"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "error: need exactly one argument\n")
		os.Exit(-1)
	}

	p, err := buildutil.PyPiVersionFromNpmVersion(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(-1)
	}

	fmt.Println(p)
}
