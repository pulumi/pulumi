// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Goal is a desired state for a resource object.
type Goal struct {
	Type       tokens.Type  // the type of resource.
	Name       tokens.QName // the name for the resource's URN.
	External   bool         // true if this resource is external to Pulumi.
	Properties PropertyMap  // the resource's property state.
	Children   []URN        // an optional list of child resource URNs.
}

// NewGoal allocates a new resource goal state.
func NewGoal(t tokens.Type, name tokens.QName, external bool, props PropertyMap, children []URN) *Goal {
	return &Goal{
		Type:       t,
		Name:       name,
		External:   external,
		Properties: props,
		Children:   children,
	}
}
