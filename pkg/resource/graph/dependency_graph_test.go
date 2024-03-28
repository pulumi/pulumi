// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package graph

import (
	"sync"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func NewProviderResource(pkg, name, id string, deps ...resource.URN) *resource.State {
	t := providers.MakeProviderType(tokens.Package(pkg))
	return &resource.State{
		Type:         t,
		URN:          resource.NewURN("test", "test", "", t, name),
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
		URN:          resource.NewURN("test", "test", "", t, name),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
		Provider:     prov,
	}
}

func TestBasicGraph(t *testing.T) {
	t.Parallel()

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
	}, dg.DependingOn(pA, nil, false))

	assert.Equal(t, []*resource.State{
		b, pB, c, d,
	}, dg.DependingOn(a, nil, false))

	assert.Equal(t, []*resource.State{
		pB, c, d,
	}, dg.DependingOn(b, nil, false))

	assert.Equal(t, []*resource.State{
		c,
	}, dg.DependingOn(pB, nil, false))

	assert.Nil(t, dg.DependingOn(c, nil, false))
	assert.Nil(t, dg.DependingOn(d, nil, false))

	pAMap := &sync.Map{}
	pAMap.Store(a.URN, true)
	pAMap.Store(b.URN, true)
	assert.Nil(t, dg.DependingOn(pA, pAMap, false))

	pAMap = &sync.Map{}
	pAMap.Store(b.URN, true)
	assert.Equal(t, []*resource.State{
		a, pB, c,
	}, dg.DependingOn(pA, pAMap, false))

	pAMap = &sync.Map{}
	pAMap.Store(a.URN, true)
	assert.Equal(t, []*resource.State{
		b, pB, c, d,
	}, dg.DependingOn(pA, pAMap, false))

	pAMap = &sync.Map{}
	pAMap.Store(b.URN, true)
	pAMap.Store(pB.URN, true)
	assert.Equal(t, []*resource.State{
		c,
	}, dg.DependingOn(a, pAMap, false))

	pAMap = &sync.Map{}
	pAMap.Store(b.URN, true)
	assert.Equal(t, []*resource.State{
		pB, c,
	}, dg.DependingOn(a, pAMap, false))

	pAMap = &sync.Map{}
	pAMap.Store(pB.URN, true)
	assert.Equal(t, []*resource.State{
		d,
	}, dg.DependingOn(b, pAMap, false))
}

// Tests that we don't add the same node to the DependingOn set twice.
func TestGraphNoDuplicates(t *testing.T) {
	t.Parallel()

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
	}, dg.DependingOn(a, nil, false))
}

func TestDependenciesOf(t *testing.T) {
	t.Parallel()

	pA := NewProviderResource("test", "pA", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA, a.URN)
	c := NewResource("c", pA)
	d := NewResource("d", pA)
	d.Parent = a.URN

	dg := NewDependencyGraph([]*resource.State{
		pA,
		a,
		b,
		c,
		d,
	})

	aDepends := dg.DependenciesOf(a)
	assert.True(t, aDepends.Contains(pA))
	assert.False(t, aDepends.Contains(a))
	assert.False(t, aDepends.Contains(b))

	bDepends := dg.DependenciesOf(b)
	assert.True(t, bDepends.Contains(pA))
	assert.True(t, bDepends.Contains(a))
	assert.False(t, bDepends.Contains(b))

	cDepends := dg.DependenciesOf(c)
	assert.True(t, cDepends.Contains(pA))
	assert.False(t, cDepends.Contains(a))
	assert.False(t, cDepends.Contains(b))

	dDepends := dg.DependenciesOf(d)
	assert.True(t, dDepends.Contains(pA))
	assert.True(t, dDepends.Contains(a)) // due to A being the parent of D
	assert.False(t, dDepends.Contains(b))
	assert.False(t, dDepends.Contains(c))
}

func TestDependenciesOfRemoteComponents(t *testing.T) {
	t.Parallel()

	aws := NewProviderResource("aws", "default", "0")
	xyz := NewProviderResource("xyz", "default", "0")
	first := NewResource("first", xyz)
	firstNested := NewResource("firstNested", xyz)
	firstNested.Parent = first.URN
	sg := NewResource("sg", aws)
	sg.Parent = firstNested.URN
	second := NewResource("second", xyz)
	rule := NewResource("rule", aws, first.URN)
	rule.Parent = second.URN

	dg := NewDependencyGraph([]*resource.State{
		aws,
		xyz,
		first,
		firstNested,
		sg,
		second,
		rule,
	})

	ruleDepends := dg.DependenciesOf(rule)
	assert.True(t, ruleDepends.Contains(first), "direct dependency")
	assert.True(t, ruleDepends.Contains(firstNested), "child of dependency")
	assert.True(t, ruleDepends.Contains(sg), "transitive child of dependency")
	assert.True(t, ruleDepends.Contains(second), "parent")
	assert.True(t, ruleDepends.Contains(aws), "provider")
	assert.False(t, ruleDepends.Contains(xyz), "unrelated")
}

func TestDependenciesOfRemoteComponentsNoCycle(t *testing.T) {
	t.Parallel()

	aws := NewProviderResource("aws", "default", "0")
	parent := NewResource("parent", aws)
	r := NewResource("r", aws, parent.URN)
	child := NewResource("child", aws, r.URN)
	child.Parent = parent.URN

	dg := NewDependencyGraph([]*resource.State{
		aws,
		parent,
		r,
		child,
	})

	childDependencies := dg.DependenciesOf(child)
	assert.True(t, childDependencies.Contains(aws))
	assert.True(t, childDependencies.Contains(parent))
	assert.True(t, childDependencies.Contains(r))

	rDependencies := dg.DependenciesOf(r)
	assert.True(t, rDependencies.Contains(aws))
	assert.True(t, rDependencies.Contains(parent))
	assert.False(t, rDependencies.Contains(child))
}

func TestTransitiveDependenciesOf(t *testing.T) {
	t.Parallel()

	aws := NewProviderResource("aws", "default", "0")
	parent := NewResource("parent", aws)
	greatUncle := NewResource("greatUncle", aws)
	uncle := NewResource("r", aws)
	uncle.Parent = greatUncle.URN
	child := NewResource("child", aws, uncle.URN)
	child.Parent = parent.URN
	baby := NewResource("baby", aws)
	baby.Parent = child.URN

	dg := NewDependencyGraph([]*resource.State{
		aws,
		parent,
		greatUncle,
		uncle,
		child,
		baby,
	})
	// <(relation)- as an alias for depends on via relation
	// baby <(Parent)- child <(Dependency)- uncle <(Parent)- greatUncle <(Provider)- aws
	set := dg.TransitiveDependenciesOf(baby)
	assert.True(t, set.Contains(aws), "everything should depend on the provider")
	assert.True(t, set.Contains(greatUncle), "child depends on greatUncle")
}

func TestOnlyDependsOn(t *testing.T) {
	t.Parallel()

	aws0 := NewProviderResource("aws", "default", "0")
	aws1 := NewProviderResource("aws", "default", "1")
	aws2 := NewProviderResource("aws", "default", "2")

	b0 := NewResource("b", aws0)
	b1 := NewResource("b", aws1)

	c0 := NewResource("c0", aws2, b0.URN)

	dg := NewDependencyGraph([]*resource.State{
		aws0,
		aws1,
		aws2,
		b0,
		b1,
		c0,
	})

	assert.Equal(t, []*resource.State{b1}, dg.OnlyDependsOn(aws1))
	assert.Equal(t, []*resource.State{c0}, dg.OnlyDependsOn(aws2))
	assert.Equal(t, []*resource.State(nil), dg.OnlyDependsOn(b0))
}

func TestOnlyDependsOnMultipleProviders(t *testing.T) {
	t.Parallel()

	aws0 := NewProviderResource("aws", "default", "0")
	aws1 := NewProviderResource("aws", "default", "1")

	b0 := NewResource("b", aws0)

	c0 := NewProviderResource("aws", "non-default", "0", b0.URN)
	c1 := NewProviderResource("aws", "non-default", "1")

	d0 := NewResource("d", c0)
	d1 := NewResource("d", c1)

	dg := NewDependencyGraph([]*resource.State{
		aws0,
		aws1,
		b0,
		c0,
		c1,
		d0,
		d1,
	})

	assert.Equal(t, []*resource.State{b0, c0, d0}, dg.OnlyDependsOn(aws0))
	assert.Equal(t, []*resource.State(nil), dg.OnlyDependsOn(aws1))
	assert.Equal(t, []*resource.State{c0, d0}, dg.OnlyDependsOn(b0))
	assert.Equal(t, []*resource.State{d1}, dg.OnlyDependsOn(c1))
}
