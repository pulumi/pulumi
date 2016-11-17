// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/diag"
)

// Phase represents a compiler phase.
type Phase interface {
	// Diag fetches the diagnostics sink used by this compiler pass.
	Diag() diag.Sink
}
