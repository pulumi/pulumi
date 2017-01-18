// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/tokens"
)

// Symbol is the base type for all MuIL symbol types.
type Symbol struct {
	Name tokens.Token // the name of this symbol.
	Tree ast.Node     // the program tree associated with this symbol.
}
