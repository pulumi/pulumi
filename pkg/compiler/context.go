// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Context holds all state available to any templates or code evaluated at compile-time.
type Context struct {
	Args   map[string]interface{}
	Target *ast.Target
}
