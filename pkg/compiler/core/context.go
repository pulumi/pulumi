// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Context holds all state available to any templates or code evaluated at compile-time.
type Context struct {
	P       map[string]interface{} // properties supplied at stack construction time.
	Cluster *ast.Cluster           // the target cluster.
}
