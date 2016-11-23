// Copyright 2016 Marapongo, Inc. All rights reserved.

package predef

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Stacks contains all of the built-in primitive types known to the Mu compiler.
var Stacks = map[ast.Name]*ast.Stack{
	MuContainer.Name:  MuContainer,
	MuGateway.Name:    MuGateway,
	MuFunc.Name:       MuFunc,
	MuEvent.Name:      MuEvent,
	MuVolume.Name:     MuVolume,
	MuAutoscaler.Name: MuAutoscaler,
	MuExtension.Name:  MuExtension,
}

const MuNamespace = "mu"

func muName(nm string) ast.Name {
	return ast.Name(MuNamespace + ast.NameDelimiter + nm)
}

var (
	MuContainer = &ast.Stack{
		Name:        muName("container"),
		Predef:      true,
		Description: "An LXC or Windows container.",
		Properties:  ast.Properties{},
	}
	MuGateway = &ast.Stack{
		Name:        muName("gateway"),
		Predef:      true,
		Description: "An API gateway and load balancer, multiplexing requests over services.",
		Properties:  ast.Properties{},
	}
	MuFunc = &ast.Stack{
		Name:        muName("func"),
		Predef:      true,
		Description: "A single standalone function for serverless scenarios.",
		Properties:  ast.Properties{},
	}
	MuEvent = &ast.Stack{
		Name:        muName("event"),
		Predef:      true,
		Description: "An event that may be used to trigger execution of another service.",
		Properties:  ast.Properties{},
	}
	MuVolume = &ast.Stack{
		Name:        muName("volume"),
		Predef:      true,
		Description: "A volume that stores data and can be mounted by other services.",
		Properties:  ast.Properties{},
	}
	MuAutoscaler = &ast.Stack{
		Name:        muName("autoscaler"),
		Predef:      true,
		Description: "A service that can automatically scale other services based on policy.",
		Properties:  ast.Properties{},
	}
	MuExtension = &ast.Stack{
		Name:        muName("extension"),
		Predef:      true,
		Description: "A logical service that extends the system by hooking system events.",
		Properties: ast.Properties{
			"provider": ast.Property{
				Name:        "provider",
				Type:        ast.PropertyTypeString,
				Description: "The name of the provider that will handle this service.",
			},
		},
	}
)
