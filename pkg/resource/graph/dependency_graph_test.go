// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package graph

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
)

func NewResource(name string, deps ...resource.URN) *resource.State {
	return &resource.State{
		Type:         tokens.Type("test"),
		URN:          resource.URN(name),
		Inputs:       make(resource.PropertyMap),
		Outputs:      make(resource.PropertyMap),
		Dependencies: deps,
	}
}

func TestBasicGraph(t *testing.T) {
	a := NewResource("a")
	b := NewResource("b", a.URN)
	c := NewResource("c", a.URN)
	d := NewResource("d", b.URN)

	dg := NewDependencyGraph([]*resource.State{
		a,
		b,
		c,
		d,
	})

	assert.Equal(t, []*resource.State{
		d, c, b,
	}, dg.DependingOn(a))

	assert.Equal(t, []*resource.State{
		d,
	}, dg.DependingOn(b))

	assert.Nil(t, dg.DependingOn(c))
	assert.Nil(t, dg.DependingOn(d))
}
