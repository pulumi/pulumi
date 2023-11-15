// Copyright 2016-2023, Pulumi Corporation.
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
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
					false, g.Dependencies, nil, g.Provider, g.PropertyDependencies, false, nil, nil, nil,
					"", false, "", nil, nil, ""),
			})
		}
		return nil
	}
}

func newTestPluginContext(t testing.TB, program deploytest.ProgramFunc) (*plugin.Context, error) {
	sink := diagtest.LogSink(t)
	statusSink := diagtest.LogSink(t)
	lang := deploytest.NewLanguageRuntime(program)
	host := deploytest.NewPluginHost(sink, statusSink, lang)
	return plugin.NewContext(sink, statusSink, host, nil, "", nil, false, nil)
}

type testProviderSource struct {
	providers map[providers.Reference]plugin.Provider
	m         sync.RWMutex
	// If nil, do not return a default provider. Otherwise, return this default provider
	defaultProvider plugin.Provider
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
	if !ok && s.defaultProvider != nil && providers.IsDefaultProvider(ref.URN()) {
		return s.defaultProvider, true
	}
	return provider, ok
}

func newProviderEvent(pkg, name string, inputs resource.PropertyMap, parent resource.URN) RegisterResourceEvent {
	if inputs == nil {
		inputs = resource.PropertyMap{}
	}
	goal := &resource.Goal{
		Type:       providers.MakeProviderType(tokens.Package(pkg)),
		ID:         "id",
		Name:       tokens.QName(name),
		Custom:     true,
		Properties: inputs,
		Parent:     parent,
	}
	return &testRegEvent{goal: goal}
}

func disableDefaultProviders(runInfo *EvalRunInfo, pkgs ...string) {
	if runInfo.Target.Config == nil {
		runInfo.Target.Config = config.Map{}
	}
	c := runInfo.Target.Config
	key := config.MustMakeKey("pulumi", "disable-default-providers")
	if _, ok, err := c.Get(key, false); err != nil {
		panic(err)
	} else if ok {
		panic("disableDefaultProviders cannot be called twice")
	}
	b, err := json.Marshal(pkgs)
	if err != nil {
		panic(err)
	}
	err = c.Set(key, config.NewValue(string(b)), false)
	if err != nil {
		panic(err)
	}
}

func TestRegisterNoDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, tokens.QName(name))
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
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		// Register a couple resources using provider A.
		&testRegEvent{
			goal: resource.NewGoal("pkgA:index:typA", "res1", true, resource.PropertyMap{}, componentURN, false, nil,
				providerARef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgA:index:typA", "res2", true, resource.PropertyMap{}, componentURN, false, nil,
				providerARef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		// Register two more providers.
		newProviderEvent("pkgA", "providerB", nil, ""),
		newProviderEvent("pkgC", "providerC", nil, componentURN),
		// Register a few resources that use the new providers.
		&testRegEvent{
			goal: resource.NewGoal("pkgB:index:typB", "res3", true, resource.PropertyMap{}, "", false, nil,
				providerBRef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgB:index:typC", "res4", true, resource.PropertyMap{}, "", false, nil,
				providerCRef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(t, fixedProgram(steps))
	assert.NoError(t, err)

	iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, &testProviderSource{})
	assert.NoError(t, err)

	processed := 0
	for {
		event, err := iter.Next()
		assert.NoError(t, err)

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
				false, nil, nil, nil, "", false, "", nil, nil, ""),
		})

		processed++
	}

	assert.Equal(t, len(steps), processed)
}

func TestRegisterDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, tokens.QName(name))
	}

	componentURN := newURN("component", "component", "")

	steps := []RegisterResourceEvent{
		// Register a component resource.
		&testRegEvent{
			goal: resource.NewGoal(componentURN.Type(), componentURN.Name(), false, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		// Register a couple resources from package A.
		&testRegEvent{
			goal: resource.NewGoal("pkgA:m:typA", "res1", true, resource.PropertyMap{},
				componentURN, false, nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgA:m:typA", "res2", true, resource.PropertyMap{},
				componentURN, false, nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		// Register a few resources from other packages.
		&testRegEvent{
			goal: resource.NewGoal("pkgB:m:typB", "res3", true, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgB:m:typC", "res4", true, resource.PropertyMap{}, "", false,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, false, "", ""),
		},
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(t, fixedProgram(steps))
	assert.NoError(t, err)

	iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, &testProviderSource{})
	assert.NoError(t, err)

	processed, defaults := 0, make(map[string]struct{})
	for {
		event, err := iter.Next()
		assert.NoError(t, err)

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
				false, nil, nil, nil, "", false, "", nil, nil, ""),
		})

		processed++
	}

	assert.Equal(t, len(steps)+len(defaults), processed)
}

func TestReadInvokeNoDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, tokens.QName(name))
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
		_, _, perr := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, providerARef.String(), "", "")
		assert.NoError(t, perr)
		_, _, perr = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, providerBRef.String(), "", "")
		assert.NoError(t, perr)
		_, _, perr = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, providerCRef.String(), "", "")
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
	ctx, err := newTestPluginContext(t, program)
	assert.NoError(t, err)

	iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
	assert.NoError(t, err)

	reads := 0
	for {
		event, err := iter.Next()
		assert.NoError(t, err)
		if event == nil {
			break
		}

		read := event.(ReadResourceEvent)
		urn := newURN(read.Type(), string(read.Name()), read.Parent())
		read.Done(&ReadResult{
			State: resource.NewState(read.Type(), urn, true, false, read.ID(), read.Properties(),
				resource.PropertyMap{}, read.Parent(), false, false, read.Dependencies(), nil, read.Provider(), nil,
				false, nil, nil, nil, "", false, "", nil, nil, ""),
		})
		reads++
	}

	assert.Equal(t, expectedReads, reads)
	assert.Equal(t, expectedInvokes, int(invokes))
}

func TestReadInvokeDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, tokens.QName(name))
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
		_, _, err := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, "", "", "")
		assert.NoError(t, err)
		_, _, err = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, "", "", "")
		assert.NoError(t, err)
		_, _, err = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, "", "", "")
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
	ctx, err := newTestPluginContext(t, program)
	assert.NoError(t, err)

	providerSource := &testProviderSource{providers: make(map[providers.Reference]plugin.Provider)}

	iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
	assert.NoError(t, err)

	reads, registers := 0, 0
	for {
		event, err := iter.Next()
		assert.NoError(t, err)

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
					false, nil, nil, nil, "", false, "", nil, nil, ""),
			})
			registers++

		case ReadResourceEvent:
			urn := newURN(e.Type(), string(e.Name()), e.Parent())
			e.Done(&ReadResult{
				State: resource.NewState(e.Type(), urn, true, false, e.ID(), e.Properties(),
					resource.PropertyMap{}, e.Parent(), false, false, e.Dependencies(), nil, e.Provider(), nil, false,
					nil, nil, nil, "", false, "", nil, nil, ""),
			})
			reads++
		}
	}

	assert.Equal(t, len(providerSource.providers), registers)
	assert.Equal(t, expectedReads, reads)
	assert.Equal(t, expectedInvokes, int(invokes))
}

// Test that we can run operations with default providers disabled.
//
// We run against the matrix of
// - enabled  vs disabled
// - explicit vs default
//
// B exists as a sanity check, to ensure that we can still perform arbitrary
// operations that belong to other packages.
func TestDisableDefaultProviders(t *testing.T) {
	t.Parallel()

	type TT struct {
		disableDefault bool
		hasExplicit    bool
		expectFail     bool
	}
	cases := []TT{}
	for _, disableDefault := range []bool{true, false} {
		for _, hasExplicit := range []bool{true, false} {
			cases = append(cases, TT{
				disableDefault: disableDefault,
				hasExplicit:    hasExplicit,
				expectFail:     disableDefault && !hasExplicit,
			})
		}
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, tt := range cases {
		tt := tt
		var name []string
		if tt.disableDefault {
			name = append(name, "disableDefault")
		}
		if tt.hasExplicit {
			name = append(name, "hasExplicit")
		}
		if tt.expectFail {
			name = append(name, "expectFail")
		}
		if len(name) == 0 {
			name = append(name, "vanilla")
		}

		t.Run(strings.Join(name, "+"), func(t *testing.T) {
			t.Parallel()

			runInfo := &EvalRunInfo{
				Proj:   &workspace.Project{Name: "test"},
				Target: &Target{Name: tokens.MustParseStackName("test")},
			}
			if tt.disableDefault {
				disableDefaultProviders(runInfo, "pkgA")
			}

			newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
				var pt tokens.Type
				if parent != "" {
					pt = parent.Type()
				}
				return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, tokens.QName(name))
			}

			newProviderURN := func(pkg tokens.Package, name string, parent resource.URN) resource.URN {
				return newURN(providers.MakeProviderType(pkg), name, parent)
			}

			providerARef, err := providers.NewReference(newProviderURN("pkgA", "providerA", ""), "id1")
			assert.NoError(t, err)
			providerBRef, err := providers.NewReference(newProviderURN("pkgB", "providerB", ""), "id2")
			assert.NoError(t, err)

			expectedReads, expectedInvokes, expectedRegisters := 3, 3, 1
			reads, invokes, registers := 0, int32(0), 0

			if tt.expectFail {
				expectedReads--
				expectedInvokes--
			}
			if !tt.hasExplicit && !tt.disableDefault && !tt.expectFail {
				// The register is creating the default provider
				expectedRegisters++
			}

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
				},
				defaultProvider: noopProvider,
			}

			program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
				aErrorAssert := assert.NoError
				if tt.expectFail {
					aErrorAssert = assert.Error
				}
				var aPkgProvider string
				if tt.hasExplicit {
					aPkgProvider = providerARef.String()
				}
				// Perform some reads and invokes with explicit provider references.
				_, _, perr := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, aPkgProvider, "", "")
				aErrorAssert(t, perr)
				_, _, perr = resmon.ReadResource("pkgB:m:typB", "resB", "id1", "", nil, providerBRef.String(), "", "")
				assert.NoError(t, perr)
				_, _, perr = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, "", "", "")
				assert.NoError(t, perr)

				_, _, perr = resmon.Invoke("pkgA:m:funcA", nil, aPkgProvider, "")
				aErrorAssert(t, perr)
				_, _, perr = resmon.Invoke("pkgB:m:funcB", nil, providerBRef.String(), "")
				assert.NoError(t, perr)
				_, _, perr = resmon.Invoke("pkgC:m:funcC", nil, "", "")
				assert.NoError(t, perr)

				return nil
			}

			// Create and iterate an eval source.
			ctx, err := newTestPluginContext(t, program)
			assert.NoError(t, err)

			iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
			assert.NoError(t, err)

			for {
				event, err := iter.Next()
				assert.NoError(t, err)
				if event == nil {
					break
				}
				switch event := event.(type) {
				case ReadResourceEvent:
					urn := newURN(event.Type(), string(event.Name()), event.Parent())
					event.Done(&ReadResult{
						State: resource.NewState(event.Type(), urn, true, false, event.ID(), event.Properties(),
							resource.PropertyMap{}, event.Parent(), false, false, event.Dependencies(), nil, event.Provider(), nil,
							false, nil, nil, nil, "", false, "", nil, nil, ""),
					})
					reads++
				case RegisterResourceEvent:
					urn := newURN(event.Goal().Type, string(event.Goal().Name), event.Goal().Parent)
					event.Done(&RegisterResult{
						State: resource.NewState(event.Goal().Type, urn, true, false, "id", event.Goal().Properties,
							resource.PropertyMap{}, event.Goal().Parent, false, false, event.Goal().Dependencies, nil,
							event.Goal().Provider, nil, false, nil, nil, nil, "", false, "", nil, nil, ""),
					})
					registers++
				default:
					panic(event)
				}
			}

			assert.Equalf(t, expectedReads, reads, "Reads")
			assert.Equalf(t, expectedInvokes, int(invokes), "Invokes")
			assert.Equalf(t, expectedRegisters, registers, "Registers")
		})
	}
}

// Validates that a resource monitor appropriately propagates
// resource options from a RegisterResourceRequest to a Construct call
// for the remote component resource (MLC).
func TestResouceMonitor_remoteComponentResourceOptions(t *testing.T) {
	t.Parallel()

	// Helper to keep a some test cases simple.
	// Takes a pointer to a container (slice or map)
	// and sets it to nil if it's empty.
	nilIfEmpty := func(s any) {
		// The code below is roughly equivalent to:
		//      if len(*s) == 0 {
		//              *s = nil
		//      }
		v := reflect.ValueOf(s) // *T for some T = []T or map[T]*
		v = v.Elem()            // *T -> T
		if v.Len() == 0 {
			// Zero value of a slice or map is nil.
			v.Set(reflect.Zero(v.Type()))
		}
	}

	runInfo := &EvalRunInfo{
		Proj:   &workspace.Project{Name: "test"},
		Target: &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, tokens.QName(name))
	}

	// Used when we need a *bool.
	trueValue, falseValue := true, false

	tests := []struct {
		desc string
		give deploytest.ResourceOptions
		want plugin.ConstructOptions
	}{
		{
			desc: "AdditionalSecretOutputs",
			give: deploytest.ResourceOptions{
				AdditionalSecretOutputs: []resource.PropertyKey{"foo"},
			},
			want: plugin.ConstructOptions{
				AdditionalSecretOutputs: []string{"foo"},
			},
		},
		{
			desc: "CustomTimeouts/Create",
			give: deploytest.ResourceOptions{
				CustomTimeouts: &resource.CustomTimeouts{Create: 5},
			},
			want: plugin.ConstructOptions{
				CustomTimeouts: &plugin.CustomTimeouts{Create: "5s"},
			},
		},
		{
			desc: "CustomTimeouts/Update",
			give: deploytest.ResourceOptions{
				CustomTimeouts: &resource.CustomTimeouts{Update: 1},
			},
			want: plugin.ConstructOptions{
				CustomTimeouts: &plugin.CustomTimeouts{Update: "1s"},
			},
		},
		{
			desc: "CustomTimeouts/Delete",
			give: deploytest.ResourceOptions{
				CustomTimeouts: &resource.CustomTimeouts{Delete: 3},
			},
			want: plugin.ConstructOptions{
				CustomTimeouts: &plugin.CustomTimeouts{Delete: "3s"},
			},
		},
		{
			desc: "DeleteBeforeReplace/true",
			give: deploytest.ResourceOptions{
				DeleteBeforeReplace: &trueValue,
			},
			want: plugin.ConstructOptions{
				DeleteBeforeReplace: true,
			},
		},
		{
			desc: "DeleteBeforeReplace/false",
			give: deploytest.ResourceOptions{
				DeleteBeforeReplace: &falseValue,
			},
			want: plugin.ConstructOptions{
				DeleteBeforeReplace: false,
			},
		},
		{
			desc: "DeletedWith",
			give: deploytest.ResourceOptions{
				DeletedWith: newURN("pkgA:m:typB", "resB", ""),
			},
			want: plugin.ConstructOptions{
				DeletedWith: newURN("pkgA:m:typB", "resB", ""),
			},
		},
		{
			desc: "IgnoreChanges",
			give: deploytest.ResourceOptions{
				IgnoreChanges: []string{"foo"},
			},
			want: plugin.ConstructOptions{
				IgnoreChanges: []string{"foo"},
			},
		},
		{
			desc: "Protect",
			give: deploytest.ResourceOptions{
				Protect: true,
			},
			want: plugin.ConstructOptions{
				Protect: true,
			},
		},
		{
			desc: "ReplaceOnChanges",
			give: deploytest.ResourceOptions{
				ReplaceOnChanges: []string{"foo"},
			},
			want: plugin.ConstructOptions{
				ReplaceOnChanges: []string{"foo"},
			},
		},
		{
			desc: "RetainOnDelete",
			give: deploytest.ResourceOptions{
				RetainOnDelete: true,
			},
			want: plugin.ConstructOptions{
				RetainOnDelete: true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			give := tt.give
			give.Remote = true
			program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
				_, _, _, err := resmon.RegisterResource("pkgA:m:typA", "resA", false, give)
				require.NoError(t, err, "register resource")
				return nil
			}
			pluginCtx, err := newTestPluginContext(t, program)
			require.NoError(t, err, "build plugin context")

			evalSource := NewEvalSource(pluginCtx, runInfo, nil, false)
			defer func() {
				assert.NoError(t, evalSource.Close(), "close eval source")
			}()

			var got plugin.ConstructOptions
			provider := &deploytest.Provider{
				ConstructF: func(
					mon *deploytest.ResourceMonitor,
					typ, name string,
					parent resource.URN,
					inputs resource.PropertyMap,
					info plugin.ConstructInfo,
					options plugin.ConstructOptions,
				) (plugin.ConstructResult, error) {
					// To keep test cases above simple,
					// nil out properties that are empty when unset.
					nilIfEmpty(&options.Aliases)
					nilIfEmpty(&options.Dependencies)
					nilIfEmpty(&options.PropertyDependencies)
					nilIfEmpty(&options.Providers)

					got = options
					return plugin.ConstructResult{
						URN: newURN(tokens.Type(typ), name, parent),
					}, nil
				},
			}

			ctx := context.Background()
			iter, res := evalSource.Iterate(ctx, Options{}, &testProviderSource{defaultProvider: provider})
			require.Nil(t, res, "iterate eval source")

			for ev, res := iter.Next(); ev != nil; ev, res = iter.Next() {
				require.Nil(t, res, "iterate eval source")
				switch ev := ev.(type) {
				case RegisterResourceEvent:
					goal := ev.Goal()
					id := goal.ID
					if id == "" {
						id = "id"
					}
					ev.Done(&RegisterResult{
						State: &resource.State{
							Type:         goal.Type,
							URN:          newURN(goal.Type, string(goal.Name), goal.Parent),
							Custom:       goal.Custom,
							ID:           id,
							Inputs:       goal.Properties,
							Parent:       goal.Parent,
							Dependencies: goal.Dependencies,
							Provider:     goal.Provider,
						},
					})
				default:
					t.Fatalf("unexpected event: %#v", ev)
				}
			}

			require.NotNil(t, got, "Provider.Construct was not called")
			assert.Equal(t, tt.want, got, "Provider.Construct options")
		})
	}
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

// 	iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
// 	assert.NoError(t, err)
// 	registrations, reads := 0, 0
// 	for {
// 		event, err := iter.Next()
// 		assert.NoError(t, err)

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

// 	iter, err := NewEvalSource(ctx, runInfo, nil, false).Iterate(context.Background(), Options{}, providerSource)
// 	assert.NoError(t, err)
// 	registered181, registered182 := false, false
// 	for {
// 		event, err := iter.Next()
// 		assert.NoError(t, err)

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

func TestResourceInheritsOptionsFromParent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		parentDeletedWith resource.URN
		childDeletedWith  resource.URN
		wantDeletedWith   resource.URN
	}{
		{
			// Children missing DeletedWith should inherit DeletedWith
			name:              "inherit",
			parentDeletedWith: "parent-deleted-with",
			childDeletedWith:  "",
			wantDeletedWith:   "parent-deleted-with",
		},
		{
			// Children with DeletedWith should not inherit DeletedWith
			name:              "override",
			parentDeletedWith: "parent-deleted-with",
			childDeletedWith:  "this-value-is-set-and-should-not-change",
			wantDeletedWith:   "this-value-is-set-and-should-not-change",
		},
		{
			// Children with DeletedWith should not inherit empty DeletedWith.
			name:              "keep",
			parentDeletedWith: "",
			childDeletedWith:  "this-value-is-set-and-should-not-change",
			wantDeletedWith:   "this-value-is-set-and-should-not-change",
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parentURN := resource.NewURN("a", "proj", "d:e:f", "a:b:c", "parent")
			parentGoal := &resource.Goal{
				Parent:      "",
				Type:        parentURN.Type(),
				DeletedWith: test.parentDeletedWith,
			}

			childURN := resource.NewURN("a", "proj", "d:e:f", "a:b:c", "child")
			goal := &resource.Goal{
				Parent:      parentURN,
				Type:        childURN.Type(),
				Name:        childURN.Name(),
				DeletedWith: test.childDeletedWith,
			}

			newGoal := inheritFromParent(*goal, *parentGoal)

			assert.Equal(t, test.wantDeletedWith, newGoal.DeletedWith)
		})
	}
}

func TestRequestFromNodeJS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	newContext := func(md map[string]string) context.Context {
		return metadata.NewIncomingContext(ctx, metadata.New(md))
	}

	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "no metadata",
			ctx:      ctx,
			expected: false,
		},
		{
			name:     "empty metadata",
			ctx:      newContext(map[string]string{}),
			expected: false,
		},
		{
			name:     "user-agent foo/1.0",
			ctx:      newContext(map[string]string{"user-agent": "foo/1.0"}),
			expected: false,
		},
		{
			name:     "user-agent grpc-node-js/1.8.15",
			ctx:      newContext(map[string]string{"user-agent": "grpc-node-js/1.8.15"}),
			expected: true,
		},
		{
			name:     "pulumi-runtime foo",
			ctx:      newContext(map[string]string{"pulumi-runtime": "foo"}),
			expected: false,
		},
		{
			name:     "pulumi-runtime nodejs",
			ctx:      newContext(map[string]string{"pulumi-runtime": "nodejs"}),
			expected: true,
		},
		{
			// Always respect the value of pulumi-runtime, regardless of the user-agent.
			name: "user-agent grpc-go/1.54.0, pulumi-runtime nodejs",
			ctx: newContext(map[string]string{
				"user-agent":     "grpc-go/1.54.0",
				"pulumi-runtime": "nodejs",
			}),
			expected: true,
		},
		{
			name: "user-agent grpc-node-js/1.8.15, pulumi-runtime python",
			ctx: newContext(map[string]string{
				"user-agent":     "grpc-node-js/1.8.15",
				"pulumi-runtime": "python",
			}),
			expected: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := requestFromNodeJS(tt.ctx)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestTransformAliasForNodeJSCompat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    resource.Alias
		expected resource.Alias
	}{
		{
			name:     `{Parent: "", NoParent: true} (transformed)`,
			input:    resource.Alias{Parent: "", NoParent: true},
			expected: resource.Alias{Parent: "", NoParent: false},
		},
		{
			name:     `{Parent: "", NoParent: false} (transformed)`,
			input:    resource.Alias{Parent: "", NoParent: false},
			expected: resource.Alias{Parent: "", NoParent: true},
		},
		{
			name:     `{Parent: "", NoParent: false, Name: "name"} (transformed)`,
			input:    resource.Alias{Parent: "", NoParent: false, Name: "name"},
			expected: resource.Alias{Parent: "", NoParent: true, Name: "name"},
		},
		{
			name:     `{Parent: "", NoParent: true, Name: "name"} (transformed)`,
			input:    resource.Alias{Parent: "", NoParent: true, Name: "name"},
			expected: resource.Alias{Parent: "", NoParent: false, Name: "name"},
		},
		{
			name:     `{Parent: "foo", NoParent: false} (no transform)`,
			input:    resource.Alias{Parent: "foo", NoParent: false},
			expected: resource.Alias{Parent: "foo", NoParent: false},
		},
		{
			name:     `{Parent: "foo", NoParent: false, Name: "name"} (no transform)`,
			input:    resource.Alias{Parent: "foo", NoParent: false, Name: "name"},
			expected: resource.Alias{Parent: "foo", NoParent: false, Name: "name"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := transformAliasForNodeJSCompat(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
