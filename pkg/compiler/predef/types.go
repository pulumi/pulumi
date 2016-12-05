// Copyright 2016 Marapongo, Inc. All rights reserved.

package predef

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Stacks contains all of the built-in primitive types known to the Mu compiler.
var Stacks = map[ast.Name]*ast.Stack{
	Container.Name:  Container,
	Gateway.Name:    Gateway,
	Func.Name:       Func,
	Event.Name:      Event,
	Volume.Name:     Volume,
	Autoscaler.Name: Autoscaler,
}

const Namespace = "mu"

func muName(nm string) ast.Name {
	return ast.Name(Namespace + ast.NameDelimiter + nm)
}

var (
	Container = &ast.Stack{
		Name:        muName("container"),
		Intrinsic:   true,
		Description: "An LXC or Windows container.",
		Properties:  ast.Properties{},
	}
	Gateway = &ast.Stack{
		Name:        muName("gateway"),
		Intrinsic:   true,
		Description: "An API gateway and load balancer, multiplexing requests over services.",
		Properties:  ast.Properties{},
	}
	Func = &ast.Stack{
		Name:        muName("func"),
		Intrinsic:   true,
		Description: "A single standalone function for serverless scenarios.",
		Properties:  ast.Properties{},
	}
	Event = &ast.Stack{
		Name:        muName("event"),
		Intrinsic:   true,
		Description: "An event that may be used to trigger execution of another service.",
		Properties:  ast.Properties{},
	}
	Volume = &ast.Stack{
		Name:        muName("volume"),
		Intrinsic:   true,
		Description: "A volume that stores data and can be mounted by other services.",
		Properties:  ast.Properties{},
	}
	Autoscaler = &ast.Stack{
		Name:        muName("autoscaler"),
		Intrinsic:   true,
		Description: "A service that can automatically scale other services based on policy.",
		Properties:  ast.Properties{},
	}
)
