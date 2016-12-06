// Copyright 2016 Marapongo, Inc. All rights reserved.

package predef

import (
	"github.com/marapongo/mu/pkg/ast"
)

const namespace = "mu"

const (
	Autoscaler = namespace + ast.NameDelimiter + "autoscaler"
	Container  = namespace + ast.NameDelimiter + "container"
	Gateway    = namespace + ast.NameDelimiter + "gateway"
	Func       = namespace + ast.NameDelimiter + "func"
	Event      = namespace + ast.NameDelimiter + "event"
	Volume     = namespace + ast.NameDelimiter + "volume"
)
