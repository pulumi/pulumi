// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
)

// PredefStackTypes contains all of the built-in primitive types known to the Mu compiler.
var PredefStackTypes = map[ast.Name]*ast.Stack{
	PredefStackMuContainer.Name:  PredefStackMuContainer,
	PredefStackMuGateway.Name:    PredefStackMuGateway,
	PredefStackMuFunc.Name:       PredefStackMuFunc,
	PredefStackMuEvent.Name:      PredefStackMuEvent,
	PredefStackMuVolume.Name:     PredefStackMuVolume,
	PredefStackMuAutoscaler.Name: PredefStackMuAutoscaler,
	PredefStackMuExtension.Name:  PredefStackMuExtension,
}

const PredefStackMuNamespace = "mu"

func muName(nm string) ast.Name {
	return ast.Name(PredefStackMuNamespace + ast.NameDelimiter + nm)
}

var (
	PredefStackMuContainer = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("container"),
			Description: "An LXC or Windows container.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
	PredefStackMuGateway = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("gateway"),
			Description: "An API gateway and load balancer, multiplexing requests over services.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
	PredefStackMuFunc = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("func"),
			Description: "A single standalone function for serverless scenarios.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
	PredefStackMuEvent = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("event"),
			Description: "An event that may be used to trigger execution of another service.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
	PredefStackMuVolume = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("volume"),
			Description: "A volume that stores data and can be mounted by other services.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
	PredefStackMuAutoscaler = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("autoscaler"),
			Description: "A service that can automatically scale other services based on policy.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
	PredefStackMuExtension = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("extension"),
			Description: "A logical service that extends the system by hooking system events.",
			Kind:        "Stack",
		},
		Parameters: ast.Parameters{},
	}
)
