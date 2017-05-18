// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmdutil

import (
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/diag"
)

var snk diag.Sink

// Sink lazily allocates a sink to be used if we can't create a compiler.
func Sink() diag.Sink {
	if snk == nil {
		snk = core.DefaultSink("")
	}
	return snk
}
