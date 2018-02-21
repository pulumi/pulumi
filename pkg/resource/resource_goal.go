// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Goal is a desired state for a resource object.
type Goal struct {
	Type         tokens.Type  // the type of resource.
	Name         tokens.QName // the name for the resource's URN.
	Custom       bool         // true if this resource is custom, managed by a plugin.
	Properties   PropertyMap  // the resource's property state.
	Parent       URN          // an optional parent URN for this resource.
	Protect      bool         // true to protect this resource from deletion.
	Dependencies []URN        // dependencies of this resource object.
}

// NewGoal allocates a new resource goal state.
func NewGoal(t tokens.Type, name tokens.QName, custom bool, props PropertyMap,
	parent URN, protect bool, dependencies []URN) *Goal {
	return &Goal{
		Type:         t,
		Name:         name,
		Custom:       custom,
		Properties:   props,
		Parent:       parent,
		Protect:      protect,
		Dependencies: dependencies,
	}
}
