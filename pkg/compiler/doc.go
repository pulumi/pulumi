// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
)

type Document struct {
	File  string     // the file that this document refers to.
	Stack *ast.Stack // the root stack element inside of this document.
}
