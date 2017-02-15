// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"fmt"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Symbol is the base interface for all MuIL symbol types.
type Symbol interface {
	Name() tokens.Name   // the simple name for this symbol.
	Token() tokens.Token // the unique qualified name token for this symbol.
	Special() bool       // indicates whether this is a "special" symbol; these are inaccessible to user code.
	Tree() diag.Diagable // the diagnosable tree associated with this symbol.
	String() string      // implement Stringer for easy formatting (e.g., in error messages).
}

var _ fmt.Stringer = (Symbol)(nil)
