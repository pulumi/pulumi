// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package graph

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
)

func NewProviderResource(pkg, name, id string, deps ...resource.URN) *resource.State {
	t := providers.MakeProviderType(tokens.Package(pkg))
	return &resource.State{
		Type:         t,
		URN:          resource.NewURN("test", "test", "", t, tokens.QName(name)),
		ID:           resource.ID(id),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
	}
}

func NewResource(name string, provider *resource.State, deps ...resource.URN) *resource.State {
	prov := ""
	if provider != nil {
		p, err := providers.NewReference(provider.URN, provider.ID)
		if err != nil {
			panic(err)
		}
		prov = p.String()
	}

	t := tokens.Type("test:test:test")
	return &resource.State{
		Type:         t,
		URN:          resource.NewURN("test", "test", "", t, tokens.QName(name)),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
		Provider:     prov,
	}
}

func TestBasicGraph(t *testing.T) {
	pA := NewProviderResource("test", "pA", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA, a.URN)
	pB := NewProviderResource("test", "pB", "1", a.URN, b.URN)
	c := NewResource("c", pB, a.URN)
	d := NewResource("d", nil, b.URN)

	dg := NewDependencyGraph([]*resource.State{
		pA,
		a,
		b,
		pB,
		c,
		d,
	})

	assert.Equal(t, []*resource.State{
		a, b, pB, c, d,
	}, dg.DependingOn(pA))

	assert.Equal(t, []*resource.State{
		b, pB, c, d,
	}, dg.DependingOn(a))

	assert.Equal(t, []*resource.State{
		pB, c, d,
	}, dg.DependingOn(b))

	assert.Equal(t, []*resource.State{
		c,
	}, dg.DependingOn(pB))

	assert.Nil(t, dg.DependingOn(c))
	assert.Nil(t, dg.DependingOn(d))
}

// Tests that we don't add the same node to the DependingOn set twice.
func TestGraphNoDuplicates(t *testing.T) {
	a := NewResource("a", nil)
	b := NewResource("b", nil, a.URN)
	c := NewResource("c", nil, a.URN)
	d := NewResource("d", nil, b.URN, c.URN)

	dg := NewDependencyGraph([]*resource.State{
		a,
		b,
		c,
		d,
	})

	assert.Equal(t, []*resource.State{
		b, c, d,
	}, dg.DependingOn(a))
}

func TestDependenciesOf(t *testing.T) {
	pA := NewProviderResource("test", "pA", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA, a.URN)
	c := NewResource("c", pA)

	dg := NewDependencyGraph([]*resource.State{
		pA,
		a,
		b,
		c,
	})

	aDepends := dg.DependenciesOf(a)
	assert.True(t, aDepends[pA])
	assert.False(t, aDepends[a])
	assert.False(t, aDepends[b])

	bDepends := dg.DependenciesOf(b)
	assert.True(t, bDepends[pA])
	assert.True(t, bDepends[a])
	assert.False(t, bDepends[b])

	cDepends := dg.DependenciesOf(c)
	assert.True(t, cDepends[pA])
	assert.False(t, cDepends[a])
	assert.False(t, cDepends[b])
}

func TestParentOf(t *testing.T) {
	a := NewResource("a", nil)
	// a.Parent is not defined - no parent.
	b := NewResource("b", nil)
	b.Parent = a.URN
	aPendingDelete := NewResource("a", nil)
	aPendingDelete.Delete = true
	bPendingDelete := NewResource("b", nil)
	bPendingDelete.Delete = true
	bPendingDelete.Parent = aPendingDelete.URN

	dg := NewDependencyGraph([]*resource.State{
		a,
		b,
		aPendingDelete,
		bPendingDelete,
	})
	assert.Equal(t, a.URN, aPendingDelete.URN)
	assert.Nil(t, dg.ParentOf(a))
	assert.Equal(t, a, dg.ParentOf(b))

	// Despite having the same URN, bPendingDelete's parent is aPendingDelete because it is the first occurrence of that
	// URN immediately above it in the snapshot.
	assert.Nil(t, dg.ParentOf(aPendingDelete))
	assert.Equal(t, aPendingDelete, dg.ParentOf(bPendingDelete))
}
