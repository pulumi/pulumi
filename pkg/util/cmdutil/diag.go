// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"os"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

var snk diag.Sink

// Diag lazily allocates a sink to be used if we can't create a compiler.
func Diag() diag.Sink {
	if snk == nil {
		snk = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Colors: true, // turn on colorization of warnings/errors.
		})
	}
	return snk
}

// InitDiag forces initialization of the diagnostics sink with the given options.
func InitDiag(opts diag.FormatOptions) {
	contract.Assertf(snk == nil, "Cannot initialize diagnostics sink more than once")
	snk = diag.DefaultSink(os.Stdout, os.Stderr, opts)
}
