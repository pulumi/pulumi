// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Goal is a desired state for a resource object.
type Goal struct {
	Type       tokens.Type  // the type of resource.
	Name       tokens.QName // the name for the resource's URN.
	Properties PropertyMap  // the resource's property state.
}

// NewGoal allocates a new resource goal state.
func NewGoal(t tokens.Type, name tokens.QName, props PropertyMap) *Goal {
	return &Goal{Type: t, Name: name, Properties: props}
}
