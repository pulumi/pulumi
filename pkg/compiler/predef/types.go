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
		Metadata: ast.Metadata{
			Name:        muName("container"),
			Description: "An LXC or Windows container.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{},
	}
	MuGateway = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("gateway"),
			Description: "An API gateway and load balancer, multiplexing requests over services.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{},
	}
	MuFunc = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("func"),
			Description: "A single standalone function for serverless scenarios.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{},
	}
	MuEvent = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("event"),
			Description: "An event that may be used to trigger execution of another service.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{},
	}
	MuVolume = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("volume"),
			Description: "A volume that stores data and can be mounted by other services.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{},
	}
	MuAutoscaler = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("autoscaler"),
			Description: "A service that can automatically scale other services based on policy.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{},
	}
	MuExtension = &ast.Stack{
		Metadata: ast.Metadata{
			Name:        muName("extension"),
			Description: "A logical service that extends the system by hooking system events.",
			Kind:        ast.MetadataKindStack,
		},
		Parameters: ast.Parameters{
			"provider": ast.Parameter{
				Name:        "provider",
				Type:        ast.ParameterTypeString,
				Description: "The name of the provider that will handle this service.",
			},
		},
	}
)
