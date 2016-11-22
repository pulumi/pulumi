// Copyright 2016 Marapongo, Inc. All rights reserved.

package schedulers

import (
	"github.com/marapongo/mu/pkg/compiler/core"
)

// Scheduler is an interface for providers that can target a Mu stack to a specific cloud CaaS.
type Scheduler interface {
	core.Backend
	Arch() Arch
}
