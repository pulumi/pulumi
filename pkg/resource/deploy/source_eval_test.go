// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type testRegEvent struct {
	goal   *resource.Goal
	result *RegisterResult
}

var _ RegisterResourceEvent = (*testRegEvent)(nil)

func (g *testRegEvent) event() {}

func (g *testRegEvent) Goal() *resource.Goal {
	return g.goal
}

func (g *testRegEvent) Done(result *RegisterResult) {
	contract.Assertf(g.result == nil, "Attempt to invoke testRegEvent.Done more than once")
	g.result = result
}

func fixedProgram(steps []RegisterResourceEvent) deploytest.ProgramFunc {
	return func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
		for _, s := range steps {
			g := s.Goal()
			urn, id, outs, err := resmon.RegisterResource(g.Type, string(g.Name), g.Custom, deploytest.ResourceOptions{
				Parent:       g.Parent,
				Protect:      g.Protect,
				Dependencies: g.Dependencies,
				Provider:     g.Provider,
				Inputs:       g.Properties,
				PropertyDeps: g.PropertyDependencies,
			})
			if err != nil {
				return err
			}
			s.Done(&RegisterResult{
				State: resource.NewState(g.Type, urn, g.Custom, false, id, g.Properties, outs, g.Parent, g.Protect,
					false, g.Dependencies, nil, g.Provider, g.PropertyDependencies, false, nil, nil, nil, ""),
			})
		}
		return nil
	}
}

func newTestPluginContext(program deploytest.ProgramFunc) (*plugin.Context, error) {
	sink := cmdutil.Diag()
	statusSink := cmdutil.Diag()
	lang := deploytest.NewLanguageRuntime(program)
	host := deploytest.NewPluginHost(sink, statusSink, lang)
	return plugin.NewContext(sink, statusSink, host, nil, "", nil, false, nil)
}

type testProviderSource struct {
	providers map[providers.Reference]plugin.Provider
	m         sync.RWMutex
}

func (s *testProviderSource) registerProvider(ref providers.Reference, provider plugin.Provider) {
	s.m.Lock()
	defer s.m.Unlock()

	s.providers[ref] = provider
}

func (s *testProviderSource) GetProvider(ref providers.Reference) (plugin.Provider, bool) {
	s.m.RLock()
	defer s.m.RUnlock()

	provider, ok := s.providers[ref]
	return provider, ok
}

func newProviderEvent(pkg, name string, inputs resource.PropertyMap, parent resource.URN) RegisterResourceEvent {
	if inputs == nil {
		inputs = resource.PropertyMap{}
	}
	goal := &resource.Goal{
		Type:       providers.MakeProviderType(tokens.Package(pkg)),
		Name:       tokens.QName(name),
		Custom:     true,
		Properties: inputs,
		Parent:     parent,
	}
	return &testRegEvent{goal: goal}
}

func TestRegisterNoDefaultProviders(t *testing.T) {
	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: "test"},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name, runInfo.Proj.Name, pt, t, tokens.QName(name))
	}

	newProviderURN := func(pkg tokens.Package, name string, parent resource.URN) resource.URN {
		return newURN(providers.MakeProviderType(pkg), name, parent)
	}

	componentURN := newURN("component", "component", "")

	providerARef, err := providers.NewReference(newProviderURN("pkgA", "providerA", ""), "id1")
	assert.NoError(t, err)
	providerBRef, err := providers.NewReference(newProviderURN("pkgA", "providerB", componentURN), "id2")
	assert.NoError(t, err)
	providerCRef, err := providers.NewReference(newProviderURN("pkgC", "providerC", ""), "id1")
	assert.NoError(t, err)

	steps := []RegisterResourceEvent{
		// Register a provider.
		newProviderEvent("pkgA", "providerA", nil, ""),
		// Register a component resource.
		&testRegEvent{
			goal: resource.NewGoal(componentURN.Type(), componentURN.Name(), false, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		// Register a couple resources using provider A.
		&testRegEvent{
			goal: resource.NewGoal("pkgA:index:typA", "res1", true, resource.PropertyMap{}, componentURN, false, nil,
				providerARef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgA:index:typA", "res2", true, resource.PropertyMap{}, componentURN, false, nil,
				providerARef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		// Register two more providers.
		newProviderEvent("pkgA", "providerB", nil, ""),
		newProviderEvent("pkgC", "providerC", nil, componentURN),
		// Register a few resources that use the new providers.
		&testRegEvent{
			goal: resource.NewGoal("pkgB:index:typB", "res3", true, resource.PropertyMap{}, "", false, nil,
				providerBRef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgB:index:typC", "res4", true, resource.PropertyMap{}, "", false, nil,
				providerCRef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(fixedProgram(steps))
	assert.NoError(t, err)

	iter, res := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, &testProviderSource{})
	assert.Nil(t, res)

	processed := 0
	for {
		event, res := iter.Next()
		assert.Nil(t, res)

		if event == nil {
			break
		}

		reg := event.(RegisterResourceEvent)

		goal := reg.Goal()
		if providers.IsProviderType(goal.Type) {
			assert.NotEqual(t, "default", goal.Name)
		}
		urn := newURN(goal.Type, string(goal.Name), goal.Parent)
		id := resource.ID("")
		if goal.Custom {
			id = "id"
		}
		reg.Done(&RegisterResult{
			State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
				goal.Parent, goal.Protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
				false, nil, nil, nil, ""),
		})

		processed++
	}

	assert.Equal(t, len(steps), processed)
}

func TestRegisterDefaultProviders(t *testing.T) {
	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: "test"},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name, runInfo.Proj.Name, pt, t, tokens.QName(name))
	}

	componentURN := newURN("component", "component", "")

	steps := []RegisterResourceEvent{
		// Register a component resource.
		&testRegEvent{
			goal: resource.NewGoal(componentURN.Type(), componentURN.Name(), false, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		// Register a couple resources from package A.
		&testRegEvent{
			goal: resource.NewGoal("pkgA:m:typA", "res1", true, resource.PropertyMap{},
				componentURN, false, nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgA:m:typA", "res2", true, resource.PropertyMap{},
				componentURN, false, nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		// Register a few resources from other packages.
		&testRegEvent{
			goal: resource.NewGoal("pkgB:m:typB", "res3", true, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgB:m:typC", "res4", true, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil),
		},
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(fixedProgram(steps))
	assert.NoError(t, err)

	iter, res := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, &testProviderSource{})
	assert.Nil(t, res)

	processed, defaults := 0, make(map[string]struct{})
	for {
		event, res := iter.Next()
		assert.Nil(t, res)

		if event == nil {
			break
		}

		reg := event.(RegisterResourceEvent)

		goal := reg.Goal()
		urn := newURN(goal.Type, string(goal.Name), goal.Parent)
		id := resource.ID("")
		if goal.Custom {
			id = "id"
		}

		if providers.IsProviderType(goal.Type) {
			assert.Equal(t, "default", string(goal.Name))
			ref, err := providers.NewReference(urn, id)
			assert.NoError(t, err)
			_, ok := defaults[ref.String()]
			assert.False(t, ok)
			defaults[ref.String()] = struct{}{}
		} else if goal.Custom {
			assert.NotEqual(t, "", goal.Provider)
			_, ok := defaults[goal.Provider]
			assert.True(t, ok)
		}

		reg.Done(&RegisterResult{
			State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
				goal.Parent, goal.Protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
				false, nil, nil, nil, ""),
		})

		processed++
	}

	assert.Equal(t, len(steps)+len(defaults), processed)
}

func TestReadInvokeNoDefaultProviders(t *testing.T) {
	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: "test"},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name, runInfo.Proj.Name, pt, t, tokens.QName(name))
	}

	newProviderURN := func(pkg tokens.Package, name string, parent resource.URN) resource.URN {
		return newURN(providers.MakeProviderType(pkg), name, parent)
	}

	providerARef, err := providers.NewReference(newProviderURN("pkgA", "providerA", ""), "id1")
	assert.NoError(t, err)
	providerBRef, err := providers.NewReference(newProviderURN("pkgA", "providerB", ""), "id2")
	assert.NoError(t, err)
	providerCRef, err := providers.NewReference(newProviderURN("pkgC", "providerC", ""), "id1")
	assert.NoError(t, err)

	invokes := int32(0)
	noopProvider := &deploytest.Provider{
		InvokeF: func(tokens.ModuleMember, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
			atomic.AddInt32(&invokes, 1)
			return resource.PropertyMap{}, nil, nil
		},
	}

	providerSource := &testProviderSource{
		providers: map[providers.Reference]plugin.Provider{
			providerARef: noopProvider,
			providerBRef: noopProvider,
			providerCRef: noopProvider,
		},
	}

	expectedReads, expectedInvokes := 3, 3
	program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
		// Perform some reads and invokes with explicit provider references.
		_, _, perr := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, providerARef.String(), "")
		assert.NoError(t, perr)
		_, _, perr = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, providerBRef.String(), "")
		assert.NoError(t, perr)
		_, _, perr = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, providerCRef.String(), "")
		assert.NoError(t, perr)

		_, _, perr = resmon.Invoke("pkgA:m:funcA", nil, providerARef.String(), "")
		assert.NoError(t, perr)
		_, _, perr = resmon.Invoke("pkgA:m:funcB", nil, providerBRef.String(), "")
		assert.NoError(t, perr)
		_, _, perr = resmon.Invoke("pkgC:m:funcC", nil, providerCRef.String(), "")
		assert.NoError(t, perr)

		return nil
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(program)
	assert.NoError(t, err)

	iter, res := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
	assert.Nil(t, res)

	reads := 0
	for {
		event, res := iter.Next()
		assert.Nil(t, res)
		if event == nil {
			break
		}

		read := event.(ReadResourceEvent)
		urn := newURN(read.Type(), string(read.Name()), read.Parent())
		read.Done(&ReadResult{
			State: resource.NewState(read.Type(), urn, true, false, read.ID(), read.Properties(),
				resource.PropertyMap{}, read.Parent(), false, false, read.Dependencies(), nil, read.Provider(), nil,
				false, nil, nil, nil, ""),
		})
		reads++
	}

	assert.Equal(t, expectedReads, reads)
	assert.Equal(t, expectedInvokes, int(invokes))
}

func TestReadInvokeDefaultProviders(t *testing.T) {
	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: "test"},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name, runInfo.Proj.Name, pt, t, tokens.QName(name))
	}

	invokes := int32(0)
	noopProvider := &deploytest.Provider{
		InvokeF: func(tokens.ModuleMember, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
			atomic.AddInt32(&invokes, 1)
			return resource.PropertyMap{}, nil, nil
		},
	}

	expectedReads, expectedInvokes := 3, 3
	program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
		// Perform some reads and invokes with default provider references.
		_, _, err := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, "", "")
		assert.NoError(t, err)
		_, _, err = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, "", "")
		assert.NoError(t, err)
		_, _, err = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, "", "")
		assert.NoError(t, err)

		_, _, err = resmon.Invoke("pkgA:m:funcA", nil, "", "")
		assert.NoError(t, err)
		_, _, err = resmon.Invoke("pkgA:m:funcB", nil, "", "")
		assert.NoError(t, err)
		_, _, err = resmon.Invoke("pkgC:m:funcC", nil, "", "")
		assert.NoError(t, err)

		return nil
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(program)
	assert.NoError(t, err)

	providerSource := &testProviderSource{providers: make(map[providers.Reference]plugin.Provider)}

	iter, res := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
	assert.Nil(t, res)

	reads, registers := 0, 0
	for {
		event, res := iter.Next()
		assert.Nil(t, res)

		if event == nil {
			break
		}

		switch e := event.(type) {
		case RegisterResourceEvent:
			goal := e.Goal()
			urn, id := newURN(goal.Type, string(goal.Name), goal.Parent), resource.ID("id")

			assert.True(t, providers.IsProviderType(goal.Type))
			assert.Equal(t, "default", string(goal.Name))
			ref, err := providers.NewReference(urn, id)
			assert.NoError(t, err)
			_, ok := providerSource.GetProvider(ref)
			assert.False(t, ok)
			providerSource.registerProvider(ref, noopProvider)

			e.Done(&RegisterResult{
				State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
					goal.Parent, goal.Protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
					false, nil, nil, nil, ""),
			})
			registers++

		case ReadResourceEvent:
			urn := newURN(e.Type(), string(e.Name()), e.Parent())
			e.Done(&ReadResult{
				State: resource.NewState(e.Type(), urn, true, false, e.ID(), e.Properties(),
					resource.PropertyMap{}, e.Parent(), false, false, e.Dependencies(), nil, e.Provider(), nil, false,
					nil, nil, nil, ""),
			})
			reads++
		}
	}

	assert.Equal(t, len(providerSource.providers), registers)
	assert.Equal(t, expectedReads, reads)
	assert.Equal(t, expectedInvokes, int(invokes))
}

// TODO[pulumi/pulumi#2753]: We should re-enable these tests (and fix them up as needed) once we have a solution
// for #2753.
// func TestReadResourceAndInvokeVersion(t *testing.T) {
// 	runInfo := &EvalRunInfo{
// 		Proj:   &workspace.Project{Name: "test"},
// 		Target: &Target{Name: "test"},
// 	}

// 	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
// 		var pt tokens.Type
// 		if parent != "" {
// 			pt = parent.Type()
// 		}
// 		return resource.NewURN(runInfo.Target.Name, runInfo.Proj.Name, pt, t, tokens.QName(name))
// 	}

// 	invokes := int32(0)
// 	noopProvider := &deploytest.Provider{
// 		InvokeF: func(tokens.ModuleMember, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
// 			atomic.AddInt32(&invokes, 1)
// 			return resource.PropertyMap{}, nil, nil
// 		},
// 	}

// 	// This program is designed to trigger the instantiation of two default providers:
// 	//  1. Provider pkgA, version 0.18.0
// 	//  2. Provider pkgC, version 0.18.0
// 	program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
// 		// Triggers pkgA, v0.18.0.
// 		_, _, err := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, "", "0.18.0")
// 		assert.NoError(t, err)
// 		// Uses pkgA's already-instantiated provider.
// 		_, _, err = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, "", "0.18.0")
// 		assert.NoError(t, err)

// 		// Triggers pkgC, v0.18.0.
// 		_, _, err = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, "", "0.18.0")
// 		assert.NoError(t, err)

// 		// Uses pkgA and pkgC's already-instantiated provider.
// 		_, _, err = resmon.Invoke("pkgA:m:funcA", nil, "", "0.18.0")
// 		assert.NoError(t, err)
// 		_, _, err = resmon.Invoke("pkgA:m:funcB", nil, "", "0.18.0")
// 		assert.NoError(t, err)
// 		_, _, err = resmon.Invoke("pkgC:m:funcC", nil, "", "0.18.0")
// 		assert.NoError(t, err)

// 		return nil
// 	}

// 	ctx, err := newTestPluginContext(program)
// 	assert.NoError(t, err)

// 	providerSource := &testProviderSource{providers: make(map[providers.Reference]plugin.Provider)}

// 	iter, res := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
// 	assert.Nil(t, res)
// 	registrations, reads := 0, 0
// 	for {
// 		event, res := iter.Next()
// 		assert.Nil(t, res)

// 		if event == nil {
// 			break
// 		}

// 		switch e := event.(type) {
// 		case RegisterResourceEvent:
// 			goal := e.Goal()
// 			urn, id := newURN(goal.Type, string(goal.Name), goal.Parent), resource.ID("id")

// 			assert.True(t, providers.IsProviderType(goal.Type))
// 			// The name of the provider resource is derived from the version requested.
// 			assert.Equal(t, "default_0_18_0", string(goal.Name))
// 			ref, err := providers.NewReference(urn, id)
// 			assert.NoError(t, err)
// 			_, ok := providerSource.GetProvider(ref)
// 			assert.False(t, ok)
// 			providerSource.registerProvider(ref, noopProvider)

// 			e.Done(&RegisterResult{
// 				State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
// 					goal.Parent, goal.Protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
// 					false, nil),
// 			})
// 			registrations++

// 		case ReadResourceEvent:
// 			urn := newURN(e.Type(), string(e.Name()), e.Parent())
// 			e.Done(&ReadResult{
// 				State: resource.NewState(e.Type(), urn, true, false, e.ID(), e.Properties(),
// 					resource.PropertyMap{}, e.Parent(), false, false, e.Dependencies(), nil, e.Provider(), nil, false,
// 					nil),
// 			})
// 			reads++
// 		}
// 	}

// 	assert.Equal(t, 2, registrations)
// 	assert.Equal(t, 3, reads)
// 	assert.Equal(t, int32(3), invokes)
// }

// func TestRegisterResourceWithVersion(t *testing.T) {
// 	runInfo := &EvalRunInfo{
// 		Proj:   &workspace.Project{Name: "test"},
// 		Target: &Target{Name: "test"},
// 	}

// 	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
// 		var pt tokens.Type
// 		if parent != "" {
// 			pt = parent.Type()
// 		}
// 		return resource.NewURN(runInfo.Target.Name, runInfo.Proj.Name, pt, t, tokens.QName(name))
// 	}

// 	noopProvider := &deploytest.Provider{}

// 	// This program is designed to trigger the instantiation of two default providers:
// 	//  1. Provider pkgA, version 0.18.0
// 	//  2. Provider pkgC, version 0.18.0
// 	program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
// 		// Triggers pkgA, v0.18.1.
// 		_, _, _, err := resmon.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
// 			resource.PropertyMap{}, nil, false, "0.18.1", nil)
// 		assert.NoError(t, err)

// 		// Re-uses pkgA's already-instantiated provider.
// 		_, _, _, err = resmon.RegisterResource("pkgA:m:typA", "resB", true, "", false, nil, "",
// 			resource.PropertyMap{}, nil, false, "0.18.1", nil)
// 		assert.NoError(t, err)

// 		// Triggers pkgA, v0.18.2
// 		_, _, _, err = resmon.RegisterResource("pkgA:m:typA", "resB", true, "", false, nil, "",
// 			resource.PropertyMap{}, nil, false, "0.18.2", nil)
// 		assert.NoError(t, err)
// 		return nil
// 	}

// 	ctx, err := newTestPluginContext(program)
// 	assert.NoError(t, err)

// 	providerSource := &testProviderSource{providers: make(map[providers.Reference]plugin.Provider)}

// 	iter, res := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
// 	assert.Nil(t, res)
// 	registered181, registered182 := false, false
// 	for {
// 		event, res := iter.Next()
// 		assert.Nil(t, res)

// 		if event == nil {
// 			break
// 		}

// 		switch e := event.(type) {
// 		case RegisterResourceEvent:
// 			goal := e.Goal()
// 			urn, id := newURN(goal.Type, string(goal.Name), goal.Parent), resource.ID("id")

// 			if providers.IsProviderType(goal.Type) {
// 				switch goal.Name {
// 				case "default_0_18_1":
// 					assert.False(t, registered181)
// 					registered181 = true
// 				case "default_0_18_2":
// 					assert.False(t, registered182)
// 					registered182 = true
// 				}

// 				ref, err := providers.NewReference(urn, id)
// 				assert.NoError(t, err)
// 				_, ok := providerSource.GetProvider(ref)
// 				assert.False(t, ok)
// 				providerSource.registerProvider(ref, noopProvider)
// 			}

// 			e.Done(&RegisterResult{
// 				State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
// 					goal.Parent, goal.Protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
// 					false, nil),
// 			})
// 		}
// 	}

// 	assert.True(t, registered181)
// 	assert.True(t, registered182)
// }
