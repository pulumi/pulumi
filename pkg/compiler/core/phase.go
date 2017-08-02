// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package core

import (
	"github.com/pulumi/pulumi-fabric/pkg/diag"
)

// Phase represents a compiler phase.
type Phase interface {
	// Diag fetches the diagnostics sink used by this compiler pass.
	Diag() diag.Sink
}
