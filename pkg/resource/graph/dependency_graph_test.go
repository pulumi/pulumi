// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package graph

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func NewStack() *resource.State {
	return &resource.State{
		Type: resource.RootStackType,
		URN:  resource.NewURN("test", "test", "", resource.RootStackType, "Stack"),
	}
}

func NewProvider(pkg, name, id string, deps ...resource.URN) *resource.State {
	t := providers.MakeProviderType(tokens.Package(pkg))
	return &resource.State{
		Type:         t,
		Custom:       true,
		URN:          resource.NewURN("test", "test", "", t, tokens.QName(name)),
		ID:           resource.ID(id),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
	}
}

func NewChildProvider(pkg, name, id string, parent resource.URN, deps ...resource.URN) *resource.State {
	prov := NewProvider(pkg, name, id, deps...)
	prov.Parent = parent
	return prov
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
		Custom:       true,
		URN:          resource.NewURN("test", "test", "", t, tokens.QName(name)),
		Inputs:       resource.PropertyMap{},
		Outputs:      resource.PropertyMap{},
		Dependencies: deps,
		Provider:     prov,
	}
}

func NewChildResource(name string, provider *resource.State, parent resource.URN,
	deps ...resource.URN) *resource.State {

	res := NewResource(name, provider, deps...)
	res.Parent = parent
	return res
}

func NewComponent(name string, deps ...resource.URN) *resource.State {
	res := NewResource(name, nil, deps...)
	res.Custom = false
	return res
}

func NewChildComponent(name string, parent resource.URN, deps ...resource.URN) *resource.State {
	res := NewChildResource(name, nil, parent, deps...)
	res.Custom = false
	return res
}

func TestBasicGraph(t *testing.T) {
	pA := NewProvider("test", "pA", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA, a.URN)
	pB := NewProvider("test", "pB", "1", a.URN, b.URN)
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
	}, dg.DependingOn(pA, nil))

	assert.Equal(t, []*resource.State{
		b, pB, c, d,
	}, dg.DependingOn(a, nil))

	assert.Equal(t, []*resource.State{
		pB, c, d,
	}, dg.DependingOn(b, nil))

	assert.Equal(t, []*resource.State{
		c,
	}, dg.DependingOn(pB, nil))

	assert.Nil(t, dg.DependingOn(c, nil))
	assert.Nil(t, dg.DependingOn(d, nil))

	assert.Nil(t, dg.DependingOn(pA, map[resource.URN]bool{
		a.URN: true,
		b.URN: true,
	}))

	assert.Equal(t, []*resource.State{
		a, pB, c,
	}, dg.DependingOn(pA, map[resource.URN]bool{
		b.URN: true,
	}))

	assert.Equal(t, []*resource.State{
		b, pB, c, d,
	}, dg.DependingOn(pA, map[resource.URN]bool{
		a.URN: true,
	}))

	assert.Equal(t, []*resource.State{
		c,
	}, dg.DependingOn(a, map[resource.URN]bool{
		b.URN:  true,
		pB.URN: true,
	}))

	assert.Equal(t, []*resource.State{
		pB, c,
	}, dg.DependingOn(a, map[resource.URN]bool{
		b.URN: true,
	}))

	assert.Equal(t, []*resource.State{
		d,
	}, dg.DependingOn(b, map[resource.URN]bool{
		pB.URN: true,
	}))
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
	}, dg.DependingOn(a, nil))
}

func TestDependenciesOf(t *testing.T) {
	pA := NewProvider("test", "pA", "0")
	a := NewResource("a", pA)
	b := NewResource("b", pA, a.URN)
	c := NewResource("c", pA)
	d := NewChildResource("d", pA, a.URN)

	dg := NewDependencyGraph([]*resource.State{
		pA,
		a,
		b,
		c,
		d,
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

	dDepends := dg.DependenciesOf(d)
	assert.True(t, dDepends[pA])
	assert.True(t, dDepends[a]) // due to A being the parent of D
	assert.False(t, dDepends[b])
	assert.False(t, dDepends[c])
}

func TestDependenciesOfComponents(t *testing.T) {
	aws := NewProvider("aws", "default", "0")
	xyz := NewProvider("xyz", "default", "0")
	first := NewComponent("first")
	firstNested := NewChildComponent("firstNested", first.URN)
	sg := NewChildResource("sg", aws, firstNested.URN)
	second := NewComponent("second")
	rule := NewChildResource("rule", aws, second.URN, first.URN)

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
	assert.True(t, ruleDepends[first], "direct dependency")
	assert.True(t, ruleDepends[firstNested], "child of dependency")
	assert.True(t, ruleDepends[sg], "transitive child of dependency")
	assert.True(t, ruleDepends[second], "parent")
	assert.True(t, ruleDepends[aws], "provider")
	assert.False(t, ruleDepends[xyz], "unrelated")
}

func TestComponentsNoCycle(t *testing.T) {
	aws := NewProvider("aws", "default", "0")
	parent := NewComponent("parent")
	r := NewResource("r", aws, parent.URN)
	child := NewChildResource("child", aws, parent.URN, r.URN)

	dg := NewDependencyGraph([]*resource.State{
		aws,
		parent,
		r,
		child,
	})

	childDependencies := dg.DependenciesOf(child)
	assert.True(t, childDependencies[aws])
	assert.True(t, childDependencies[parent])
	assert.True(t, childDependencies[r])

	rDependencies := dg.DependenciesOf(r)
	assert.True(t, rDependencies[aws])
	assert.True(t, rDependencies[parent])
	assert.False(t, rDependencies[child])

	parentDependents := dg.DependingOn(parent, nil)
	assert.Equal(t, []*resource.State{r, child}, parentDependents)

	providerDependents := dg.DependingOn(aws, nil)
	assert.Equal(t, []*resource.State{r, child}, providerDependents)
}

func TestComponentsNoCycle2(t *testing.T) {
	aws := NewProvider("aws", "default", "0")
	parent := NewComponent("parent")
	r := NewResource("r", aws, parent.URN)
	child := NewChildResource("child", aws, parent.URN, r.URN)
	child2 := NewChildResource("child2", aws, parent.URN)

	dg := NewDependencyGraph([]*resource.State{
		aws,
		parent,
		r,
		child2,
		child,
	})

	childDependencies := dg.DependenciesOf(child)
	assert.True(t, childDependencies[aws])
	assert.True(t, childDependencies[parent])
	assert.True(t, childDependencies[r])

	rDependencies := dg.DependenciesOf(r)
	assert.True(t, rDependencies[aws])
	assert.True(t, rDependencies[parent])
	assert.False(t, rDependencies[child])
	assert.True(t, rDependencies[child2])

	child2Dependents := dg.DependingOn(child2, nil)
	assert.Equal(t, []*resource.State{r, child}, child2Dependents)

	parentDependents := dg.DependingOn(parent, nil)
	assert.Equal(t, []*resource.State{r, child2, child}, parentDependents)

	providerDependents := dg.DependingOn(aws, nil)
	assert.Equal(t, []*resource.State{r, child2, child}, providerDependents)
}

// This tests the canonical delete dependency example for the following family tree.
//
//                              <root>
//                                |
//             ___________________|_________
//             |                  |        |
//           Comp1            Provider    Sink
//      _______|__________
//      |      |          |
//   Cust1   Comp2      Comp3
//          ___|___       |
//          |     |       |
//        Cust2  Cust3  Comp4
//          |             |
//        Cust4         Cust5
//
// The only _declared_ dependency is from Sink to Comp1. Each custom resource depends on Provider.
//
func TestCanonicalExample(t *testing.T) {
	prov := NewProvider("pkg", "default", "0")
	comp1 := NewComponent("comp1")
	cust1 := NewChildResource("cust1", prov, comp1.URN)
	comp2 := NewChildComponent("comp2", comp1.URN)
	comp3 := NewChildComponent("comp3", comp1.URN)
	cust2 := NewChildResource("cust2", prov, comp2.URN)
	cust3 := NewChildResource("cust3", prov, comp2.URN)
	comp4 := NewChildComponent("comp4", comp3.URN)
	cust4 := NewChildResource("cust4", prov, cust2.URN)
	cust5 := NewChildResource("cust5", prov, comp4.URN)
	sink := NewResource("sink", prov, comp1.URN)
	sink2 := NewResource("sink2", prov, cust2.URN)

	dg := NewDependencyGraph([]*resource.State{
		prov,
		comp1,
		cust1,
		comp2,
		comp3,
		cust2,
		cust3,
		comp4,
		cust4,
		cust5,
		sink,
		sink2,
	})

	provDependencies := dg.DependenciesOf(prov)
	assert.Empty(t, provDependencies)
	provDependents := dg.DependingOn(prov, nil)
	assert.Equal(t, []*resource.State{cust1, cust2, cust3, cust4, cust5, sink, sink2}, provDependents)

	comp1Dependencies := dg.DependenciesOf(comp1)
	assert.Empty(t, comp1Dependencies)
	comp1Dependents := dg.DependingOn(comp1, nil)
	assert.Equal(t, []*resource.State{cust1, comp2, comp3, cust2, cust3, comp4, cust4, cust5, sink, sink2}, comp1Dependents)

	comp2Dependencies := dg.DependenciesOf(comp2)
	assert.Equal(t, ResourceSet{comp1: true}, comp2Dependencies)
	comp2Dependents := dg.DependingOn(comp2, nil)
	assert.Equal(t, []*resource.State{cust2, cust3, cust4, sink, sink2}, comp2Dependents)

	comp3Dependencies := dg.DependenciesOf(comp3)
	assert.Equal(t, ResourceSet{comp1: true}, comp3Dependencies)
	comp3Dependents := dg.DependingOn(comp3, nil)
	assert.Equal(t, []*resource.State{comp4, cust5, sink}, comp3Dependents)

	comp4Dependencies := dg.DependenciesOf(comp4)
	assert.Equal(t, ResourceSet{comp3: true}, comp4Dependencies)
	comp4Dependents := dg.DependingOn(comp4, nil)
	assert.Equal(t, []*resource.State{cust5, sink}, comp4Dependents)

	cust1Dependencies := dg.DependenciesOf(cust1)
	assert.Equal(t, ResourceSet{prov: true, comp1: true}, cust1Dependencies)
	cust1Dependents := dg.DependingOn(cust1, nil)
	assert.Equal(t, []*resource.State{sink}, cust1Dependents)

	cust2Dependencies := dg.DependenciesOf(cust2)
	assert.Equal(t, ResourceSet{prov: true, comp2: true}, cust2Dependencies)
	cust2Dependents := dg.DependingOn(cust2, nil)
	assert.Equal(t, []*resource.State{cust4, sink, sink2}, cust2Dependents)

	cust3Dependencies := dg.DependenciesOf(cust3)
	assert.Equal(t, ResourceSet{prov: true, comp2: true}, cust3Dependencies)
	cust3Dependents := dg.DependingOn(cust3, nil)
	assert.Equal(t, []*resource.State{sink}, cust3Dependents)

	cust4Dependencies := dg.DependenciesOf(cust4)
	assert.Equal(t, ResourceSet{prov: true, cust2: true}, cust4Dependencies)
	cust4Dependents := dg.DependingOn(cust4, nil)
	assert.Empty(t, cust4Dependents)

	cust5Dependencies := dg.DependenciesOf(cust5)
	assert.Equal(t, ResourceSet{prov: true, comp4: true}, cust5Dependencies)
	cust5Dependents := dg.DependingOn(cust5, nil)
	assert.Equal(t, []*resource.State{sink}, cust5Dependents)

	sinkDependencies := dg.DependenciesOf(sink)
	assert.Equal(t, ResourceSet{
		prov:  true,
		comp1: true,
		cust1: true,
		comp2: true,
		comp3: true,
		cust2: true,
		cust3: true,
		comp4: true,
		cust5: true,
	}, sinkDependencies)
	sinkDependents := dg.DependingOn(sink, nil)
	assert.Empty(t, sinkDependents)

	sink2Dependencies := dg.DependenciesOf(sink2)
	assert.Equal(t, ResourceSet{prov: true, cust2: true}, sink2Dependencies)
	sink2Dependents := dg.DependingOn(sink2, nil)
	assert.Empty(t, sink2Dependents)
}

func TestEKSExample(t *testing.T) {
	// Stack resource
	stack := NewStack()

	// Root providers
	aws := NewChildProvider("aws", "default", "0", stack.URN)
	eks := NewChildProvider("eks", "default", "0", stack.URN)

	// EKS component resource
	clusterComponent := NewChildComponent("eksCluster", stack.URN)
	vpc := NewChildResource("vpc", aws, clusterComponent.URN)
	subnet1 := NewChildResource("subnet1", aws, clusterComponent.URN, vpc.URN)
	subnet2 := NewChildResource("subnet2", aws, clusterComponent.URN, vpc.URN)
	eksRole := NewChildResource("eksRole", aws, clusterComponent.URN)
	secGroup := NewChildResource("secGroup", aws, clusterComponent.URN, vpc.URN)
	egressRule := NewChildResource("egressRule", aws, clusterComponent.URN, secGroup.URN)
	cluster := NewChildResource("cluster", aws, clusterComponent.URN, eksRole.URN, secGroup.URN, subnet1.URN, subnet2.URN)
	k8s := NewChildProvider("k8s", "eks", "0", clusterComponent.URN, cluster.URN)

	// Helm chart
	helmComponent := NewChildComponent("helm", stack.URN)
	namespace := NewChildResource("namespace", k8s, helmComponent.URN)
	service := NewChildResource("service", k8s, helmComponent.URN, namespace.URN)
	deployment := NewChildResource("deployment", k8s, helmComponent.URN, namespace.URN)

	// End user
	sink := NewChildResource("sink", nil, stack.URN, helmComponent.URN)

	// Dependency graph
	dg := NewDependencyGraph([]*resource.State{
		stack,
		aws,
		eks,
		clusterComponent,
		vpc,
		subnet1,
		subnet2,
		eksRole,
		secGroup,
		egressRule,
		cluster,
		k8s,
		helmComponent,
		sink,
		namespace, // These appear after sink to model the creation of the chart's resources inside of an apply
		deployment,
		service,
	})

	cases := []struct {
		resource     *resource.State
		dependencies ResourceSet
		dependents   []*resource.State
	}{
		{
			resource:     stack,
			dependencies: ResourceSet{},
			dependents: []*resource.State{
				aws,
				eks,
				clusterComponent,
				vpc,
				subnet1,
				subnet2,
				eksRole,
				secGroup,
				egressRule,
				cluster,
				k8s,
				helmComponent,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     aws,
			dependencies: ResourceSet{stack: true},
			dependents: []*resource.State{
				vpc,
				subnet1,
				subnet2,
				eksRole,
				secGroup,
				egressRule,
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     eks,
			dependencies: ResourceSet{stack: true},
		},
		{
			resource:     clusterComponent,
			dependencies: ResourceSet{stack: true},
			dependents: []*resource.State{
				vpc,
				subnet1,
				subnet2,
				eksRole,
				secGroup,
				egressRule,
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     vpc,
			dependencies: ResourceSet{aws: true, clusterComponent: true},
			dependents: []*resource.State{
				subnet1,
				subnet2,
				secGroup,
				egressRule,
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     subnet1,
			dependencies: ResourceSet{aws: true, clusterComponent: true, vpc: true},
			dependents: []*resource.State{
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     subnet2,
			dependencies: ResourceSet{aws: true, clusterComponent: true, vpc: true},
			dependents: []*resource.State{
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     eksRole,
			dependencies: ResourceSet{aws: true, clusterComponent: true},
			dependents: []*resource.State{
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     secGroup,
			dependencies: ResourceSet{aws: true, clusterComponent: true, vpc: true},
			dependents: []*resource.State{
				egressRule,
				cluster,
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     egressRule,
			dependencies: ResourceSet{aws: true, clusterComponent: true, secGroup: true},
		},
		{
			resource: cluster,
			dependencies: ResourceSet{
				aws:              true,
				clusterComponent: true,
				eksRole:          true,
				secGroup:         true,
				subnet1:          true,
				subnet2:          true,
			},
			dependents: []*resource.State{
				k8s,
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource:     k8s,
			dependencies: ResourceSet{clusterComponent: true, cluster: true},
			dependents: []*resource.State{
				sink,
				namespace,
				deployment,
				service,
			},
		},
		{
			resource: sink,
			dependencies: ResourceSet{
				stack:         true,
				helmComponent: true,
				namespace:     true,
				deployment:    true,
				service:       true,
			},
		},
		{
			resource:     namespace,
			dependencies: ResourceSet{k8s: true, helmComponent: true},
			dependents: []*resource.State{
				sink,
				deployment,
				service,
			},
		},
		{
			resource:     deployment,
			dependencies: ResourceSet{k8s: true, helmComponent: true, namespace: true},
			dependents: []*resource.State{
				sink,
			},
		},
		{
			resource:     service,
			dependencies: ResourceSet{k8s: true, helmComponent: true, namespace: true},
			dependents: []*resource.State{
				sink,
			},
		},
	}

	for _, c := range cases {
		t.Run(string(c.resource.URN.Name()), func(t *testing.T) {
			dependencies := dg.DependenciesOf(c.resource)
			assert.Equal(t, c.dependencies, dependencies)

			dependents := dg.DependingOn(c.resource, nil)
			assert.Equal(t, c.dependents, dependents)
		})
	}
}

// This test exercises a complicated case in which a resource has a proper dependency on its own parent. While these
// dependencies are legal, it is critical that component dependency expansion remains acyclic in these cases. In
// this test, the family tree is:
//
//                comp1
//       ___________|___________
//       |          |          |
//     comp2      cust1      cust2
//       |
//     cust3
//
// And the declared dependency graph is:
//
//     cust2
//       |
//       v
//     comp2
//       |
//       v
//     comp1
//
// The expanded dependency graph should be (minus provider deps):
//
//        cust2--+--+
//          |    |  |
//          V    |  |
//        cust3  |  |
//          |    |  |
//          V    |  |
//     +--comp2<-+  |
//     |    |       |
//     |    v       |
//     |  cust1<----+
//     |    |
//     |    v
//     +->comp1
//
func TestDependsOnParentComponent(t *testing.T) {
	prov := NewProvider("pkg", "default", "0")
	comp1 := NewComponent("comp1")
	comp2 := NewChildComponent("comp2", comp1.URN, comp1.URN)
	cust1 := NewChildResource("cust1", prov, comp1.URN)
	cust2 := NewChildResource("cust2", prov, comp1.URN, comp2.URN)
	cust3 := NewChildResource("cust3", prov, comp2.URN)

	dg := NewDependencyGraph([]*resource.State{
		prov,
		comp1,
		comp2,
		cust1,
		cust2,
		cust3,
	})

	cases := []struct {
		resource     *resource.State
		dependencies ResourceSet
		dependents   []*resource.State
	}{
		{
			resource:     prov,
			dependencies: ResourceSet{},
			dependents:   []*resource.State{comp2, cust1, cust2, cust3},
		},
		{
			resource:     comp1,
			dependencies: ResourceSet{},
			dependents:   []*resource.State{comp2, cust1, cust2, cust3},
		},
		{
			resource:     comp2,
			dependencies: ResourceSet{comp1: true, cust1: true},
			dependents:   []*resource.State{cust2, cust3},
		},
		{
			resource:     cust1,
			dependencies: ResourceSet{prov: true, comp1: true},
			dependents:   []*resource.State{comp2, cust2, cust3},
		},
		{
			resource:     cust2,
			dependencies: ResourceSet{prov: true, comp1: true, comp2: true, cust3: true},
		},
		{
			resource:     cust3,
			dependencies: ResourceSet{prov: true, comp2: true},
			dependents:   []*resource.State{cust2},
		},
	}

	for _, c := range cases {
		t.Run(string(c.resource.URN.Name()), func(t *testing.T) {
			dependencies := dg.DependenciesOf(c.resource)
			assert.Equal(t, c.dependencies, dependencies)

			dependents := dg.DependingOn(c.resource, nil)
			assert.Equal(t, c.dependents, dependents)
		})
	}
}

// This test is a variant of the above, but with another component added to the parent chain between comp2 and comp1.
func TestDependsOnAncestorComponent(t *testing.T) {
	prov := NewProvider("pkg", "default", "0")
	comp1 := NewComponent("comp1")
	comp2 := NewChildComponent("comp2", comp1.URN)
	comp3 := NewChildComponent("comp3", comp2.URN, comp1.URN)
	cust1 := NewChildResource("cust1", prov, comp2.URN)
	cust2 := NewChildResource("cust2", prov, comp2.URN, comp3.URN)
	cust3 := NewChildResource("cust3", prov, comp3.URN)

	dg := NewDependencyGraph([]*resource.State{
		prov,
		comp1,
		comp2,
		comp3,
		cust1,
		cust2,
		cust3,
	})

	cases := []struct {
		resource     *resource.State
		dependencies ResourceSet
		dependents   []*resource.State
	}{
		{
			resource:     prov,
			dependencies: ResourceSet{},
			dependents:   []*resource.State{comp3, cust1, cust2, cust3},
		},
		{
			resource:     comp1,
			dependencies: ResourceSet{},
			dependents:   []*resource.State{comp2, comp3, cust1, cust2, cust3},
		},
		{
			resource:     comp2,
			dependencies: ResourceSet{comp1: true},
			dependents:   []*resource.State{comp3, cust1, cust2, cust3},
		},
		{
			resource:     comp3,
			dependencies: ResourceSet{comp1: true, comp2: true, cust1: true},
			dependents:   []*resource.State{cust2, cust3},
		},
		{
			resource:     cust1,
			dependencies: ResourceSet{prov: true, comp2: true},
			dependents:   []*resource.State{comp3, cust2, cust3},
		},
		{
			resource:     cust2,
			dependencies: ResourceSet{prov: true, comp2: true, comp3: true, cust3: true},
		},
		{
			resource:     cust3,
			dependencies: ResourceSet{prov: true, comp3: true},
			dependents:   []*resource.State{cust2},
		},
	}

	for _, c := range cases {
		t.Run(string(c.resource.URN.Name()), func(t *testing.T) {
			dependencies := dg.DependenciesOf(c.resource)
			assert.Equal(t, c.dependencies, dependencies)

			dependents := dg.DependingOn(c.resource, nil)
			assert.Equal(t, c.dependents, dependents)
		})
	}
}
