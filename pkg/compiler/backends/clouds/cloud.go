// Copyright 2016 Marapongo, Inc. All rights reserved.

package clouds

import (
	"github.com/marapongo/mu/pkg/compiler/core"
)

// Cloud is an interface for providers that can target a Mu stack to a specific cloud IaaS.
type Cloud interface {
	core.Backend
	Arch() Arch
}
