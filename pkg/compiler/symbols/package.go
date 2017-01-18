// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/tokens"
)

// Package is a fully bound package symbol.
type Package struct {
	Symbol
	Modules map[tokens.Module]*Module
}
