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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/blang/semver"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

//
// Mock resource monitor.
//

type mockResmon struct {
	AddressF func() string

	CancelF func() error

	InvokeF func(ctx context.Context,
		req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error)

	CallF func(ctx context.Context,
		req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error)

	ReadResourceF func(ctx context.Context,
		req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error)

	RegisterResourceF func(ctx context.Context,
		req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)

	RegisterResourceOutputsF func(ctx context.Context,
		req *pulumirpc.RegisterResourceOutputsRequest) (*emptypb.Empty, error)

	AbortChanF func() <-chan bool
}

var _ SourceResourceMonitor = (*mockResmon)(nil)

func (rm *mockResmon) AbortChan() <-chan bool {
	if rm.AbortChanF != nil {
		return rm.AbortChanF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Address() string {
	if rm.AddressF != nil {
		return rm.AddressF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Cancel() error {
	if rm.CancelF != nil {
		return rm.CancelF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Invoke(ctx context.Context,
	req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	if rm.InvokeF != nil {
		return rm.InvokeF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) Call(ctx context.Context,
	req *pulumirpc.ResourceCallRequest,
) (*pulumirpc.CallResponse, error) {
	if rm.CallF != nil {
		return rm.CallF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	if rm.ReadResourceF != nil {
		return rm.ReadResourceF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	if rm.RegisterResourceF != nil {
		return rm.RegisterResourceF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	if rm.RegisterResourceOutputsF != nil {
		return rm.RegisterResourceOutputsF(ctx, req)
	}
	panic("not implemented")
}

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
			resp, err := resmon.RegisterResource(g.Type, g.Name, g.Custom, deploytest.ResourceOptions{
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
			var protect bool
			if g.Protect != nil {
				protect = *g.Protect
			}
			s.Done(&RegisterResult{
				State: resource.NewState(g.Type, resp.URN, g.Custom, false, resp.ID, g.Properties, resp.Outputs, g.Parent,
					protect, false, g.Dependencies, nil, g.Provider, g.PropertyDependencies, false, nil, nil, nil,
					"", false, "", nil, nil, "", nil, nil, false),
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
	return plugin.NewContext(context.Background(), sink, statusSink, host, nil, "", nil, false, nil)
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
		Name:       name,
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
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj:        &workspace.Project{Name: "test"},
		Target:      &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, name)
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
			goal: resource.NewGoal(componentURN.Type(), componentURN.Name(), false, resource.PropertyMap{}, "", nil,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		// Register a couple resources using provider A.
		&testRegEvent{
			goal: resource.NewGoal("pkgA:index:typA", "res1", true, resource.PropertyMap{}, componentURN, nil, nil,
				providerARef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgA:index:typA", "res2", true, resource.PropertyMap{}, componentURN, nil, nil,
				providerARef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		// Register two more providers.
		newProviderEvent("pkgA", "providerB", nil, ""),
		newProviderEvent("pkgC", "providerC", nil, componentURN),
		// Register a few resources that use the new providers.
		&testRegEvent{
			goal: resource.NewGoal("pkgB:index:typB", "res3", true, resource.PropertyMap{}, "", nil, nil,
				providerBRef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgB:index:typC", "res4", true, resource.PropertyMap{}, "", nil, nil,
				providerCRef.String(), []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(t, fixedProgram(steps))
	assert.NoError(t, err)

	iter, err := NewEvalSource(ctx, runInfo, nil, EvalSourceOptions{}).Iterate(context.Background(), &testProviderSource{})
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
		urn := newURN(goal.Type, goal.Name, goal.Parent)
		id := resource.ID("")
		if goal.Custom {
			id = "id"
		}
		var protect bool
		if goal.Protect != nil {
			protect = *goal.Protect
		}
		reg.Done(&RegisterResult{
			State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
				goal.Parent, protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
				false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
		})

		processed++
	}

	assert.Equal(t, len(steps), processed)
}

func TestRegisterDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj:        &workspace.Project{Name: "test"},
		Target:      &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, name)
	}

	componentURN := newURN("component", "component", "")

	steps := []RegisterResourceEvent{
		// Register a component resource.
		&testRegEvent{
			goal: resource.NewGoal(componentURN.Type(), componentURN.Name(), false, resource.PropertyMap{}, "", nil,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		// Register a couple resources from package A.
		&testRegEvent{
			goal: resource.NewGoal("pkgA:m:typA", "res1", true, resource.PropertyMap{},
				componentURN, nil, nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgA:m:typA", "res2", true, resource.PropertyMap{},
				componentURN, nil, nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		// Register a few resources from other packages.
		&testRegEvent{
			goal: resource.NewGoal("pkgB:m:typB", "res3", true, resource.PropertyMap{}, "", nil,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
		&testRegEvent{
			goal: resource.NewGoal("pkgB:m:typC", "res4", true, resource.PropertyMap{}, "", nil,
				nil, "", []string{}, nil, nil, nil, nil, nil, "", nil, nil, nil, "", ""),
		},
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(t, fixedProgram(steps))
	assert.NoError(t, err)

	iter, err := NewEvalSource(ctx, runInfo, nil, EvalSourceOptions{}).Iterate(context.Background(), &testProviderSource{})
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
		urn := newURN(goal.Type, goal.Name, goal.Parent)
		id := resource.ID("")
		if goal.Custom {
			id = "id"
		}

		if providers.IsProviderType(goal.Type) {
			assert.Equal(t, "default", goal.Name)
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

		var protect bool
		if goal.Protect != nil {
			protect = *goal.Protect
		}
		reg.Done(&RegisterResult{
			State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
				goal.Parent, protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
				false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
		})

		processed++
	}

	assert.Equal(t, len(steps)+len(defaults), processed)
}

func TestReadInvokeNoDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj:        &workspace.Project{Name: "test"},
		Target:      &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, name)
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
		InvokeF: func(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
			atomic.AddInt32(&invokes, 1)
			return plugin.InvokeResponse{}, nil
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
		_, _, perr := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, providerARef.String(), "", "", "")
		assert.NoError(t, perr)
		_, _, perr = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, providerBRef.String(), "", "", "")
		assert.NoError(t, perr)
		_, _, perr = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, providerCRef.String(), "", "", "")
		assert.NoError(t, perr)

		_, _, perr = resmon.Invoke("pkgA:m:funcA", nil, providerARef.String(), "", "")
		assert.NoError(t, perr)
		_, _, perr = resmon.Invoke("pkgA:m:funcB", nil, providerBRef.String(), "", "")
		assert.NoError(t, perr)
		_, _, perr = resmon.Invoke("pkgC:m:funcC", nil, providerCRef.String(), "", "")
		assert.NoError(t, perr)

		return nil
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(t, program)
	assert.NoError(t, err)

	iter, err := NewEvalSource(ctx, runInfo, nil, EvalSourceOptions{}).Iterate(context.Background(), providerSource)
	assert.NoError(t, err)

	reads := 0
	for {
		event, err := iter.Next()
		assert.NoError(t, err)
		if event == nil {
			break
		}

		read := event.(ReadResourceEvent)
		urn := newURN(read.Type(), read.Name(), read.Parent())
		read.Done(&ReadResult{
			State: resource.NewState(read.Type(), urn, true, false, read.ID(), read.Properties(),
				resource.PropertyMap{}, read.Parent(), false, false, read.Dependencies(), nil, read.Provider(), nil,
				false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
		})
		reads++
	}

	assert.Equal(t, expectedReads, reads)
	assert.Equal(t, expectedInvokes, int(invokes))
}

func TestReadInvokeDefaultProviders(t *testing.T) {
	t.Parallel()

	runInfo := &EvalRunInfo{
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj:        &workspace.Project{Name: "test"},
		Target:      &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, name)
	}

	invokes := int32(0)
	noopProvider := &deploytest.Provider{
		InvokeF: func(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
			atomic.AddInt32(&invokes, 1)
			return plugin.InvokeResponse{}, nil
		},
	}

	expectedReads, expectedInvokes := 3, 3
	program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
		// Perform some reads and invokes with default provider references.
		_, _, err := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, "", "", "", "")
		assert.NoError(t, err)
		_, _, err = resmon.ReadResource("pkgA:m:typB", "resB", "id1", "", nil, "", "", "", "")
		assert.NoError(t, err)
		_, _, err = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, "", "", "", "")
		assert.NoError(t, err)

		_, _, err = resmon.Invoke("pkgA:m:funcA", nil, "", "", "")
		assert.NoError(t, err)
		_, _, err = resmon.Invoke("pkgA:m:funcB", nil, "", "", "")
		assert.NoError(t, err)
		_, _, err = resmon.Invoke("pkgC:m:funcC", nil, "", "", "")
		assert.NoError(t, err)

		return nil
	}

	// Create and iterate an eval source.
	ctx, err := newTestPluginContext(t, program)
	assert.NoError(t, err)

	providerSource := &testProviderSource{providers: make(map[providers.Reference]plugin.Provider)}

	iter, err := NewEvalSource(ctx, runInfo, nil, EvalSourceOptions{}).Iterate(context.Background(), providerSource)
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
			urn, id := newURN(goal.Type, goal.Name, goal.Parent), resource.ID("id")

			assert.True(t, providers.IsProviderType(goal.Type))
			assert.Equal(t, "default", goal.Name)
			ref, err := providers.NewReference(urn, id)
			assert.NoError(t, err)
			_, ok := providerSource.GetProvider(ref)
			assert.False(t, ok)
			providerSource.registerProvider(ref, noopProvider)

			var protect bool
			if goal.Protect != nil {
				protect = *goal.Protect
			}

			e.Done(&RegisterResult{
				State: resource.NewState(goal.Type, urn, goal.Custom, false, id, goal.Properties, resource.PropertyMap{},
					goal.Parent, protect, false, goal.Dependencies, nil, goal.Provider, goal.PropertyDependencies,
					false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
			})
			registers++

		case ReadResourceEvent:
			urn := newURN(e.Type(), e.Name(), e.Parent())
			e.Done(&ReadResult{
				State: resource.NewState(e.Type(), urn, true, false, e.ID(), e.Properties(),
					resource.PropertyMap{}, e.Parent(), false, false, e.Dependencies(), nil, e.Provider(), nil, false,
					nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
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
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "test"},
				Target:      &Target{Name: tokens.MustParseStackName("test")},
			}
			if tt.disableDefault {
				disableDefaultProviders(runInfo, "pkgA")
			}

			newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
				var pt tokens.Type
				if parent != "" {
					pt = parent.Type()
				}
				return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, name)
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
				InvokeF: func(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					atomic.AddInt32(&invokes, 1)
					return plugin.InvokeResponse{}, nil
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
				_, _, perr := resmon.ReadResource("pkgA:m:typA", "resA", "id1", "", nil, aPkgProvider, "", "", "")
				aErrorAssert(t, perr)
				_, _, perr = resmon.ReadResource("pkgB:m:typB", "resB", "id1", "", nil, providerBRef.String(), "", "", "")
				assert.NoError(t, perr)
				_, _, perr = resmon.ReadResource("pkgC:m:typC", "resC", "id1", "", nil, "", "", "", "")
				assert.NoError(t, perr)

				_, _, perr = resmon.Invoke("pkgA:m:funcA", nil, aPkgProvider, "", "")
				aErrorAssert(t, perr)
				_, _, perr = resmon.Invoke("pkgB:m:funcB", nil, providerBRef.String(), "", "")
				assert.NoError(t, perr)
				_, _, perr = resmon.Invoke("pkgC:m:funcC", nil, "", "", "")
				assert.NoError(t, perr)

				return nil
			}

			// Create and iterate an eval source.
			ctx, err := newTestPluginContext(t, program)
			assert.NoError(t, err)

			iter, err := NewEvalSource(ctx, runInfo, nil, EvalSourceOptions{}).Iterate(context.Background(), providerSource)
			assert.NoError(t, err)

			for {
				event, err := iter.Next()
				assert.NoError(t, err)
				if event == nil {
					break
				}
				switch event := event.(type) {
				case ReadResourceEvent:
					urn := newURN(event.Type(), event.Name(), event.Parent())
					event.Done(&ReadResult{
						State: resource.NewState(event.Type(), urn, true, false, event.ID(), event.Properties(),
							resource.PropertyMap{}, event.Parent(), false, false, event.Dependencies(), nil, event.Provider(), nil,
							false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
					})
					reads++
				case RegisterResourceEvent:
					urn := newURN(event.Goal().Type, event.Goal().Name, event.Goal().Parent)
					event.Done(&RegisterResult{
						State: resource.NewState(event.Goal().Type, urn, true, false, "id", event.Goal().Properties,
							resource.PropertyMap{}, event.Goal().Parent, false, false, event.Goal().Dependencies, nil,
							event.Goal().Provider, nil, false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, false),
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
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj:        &workspace.Project{Name: "test"},
		Target:      &Target{Name: tokens.MustParseStackName("test")},
	}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(runInfo.Target.Name.Q(), runInfo.Proj.Name, pt, t, name)
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
				DeleteBeforeReplace: &trueValue,
			},
		},
		{
			desc: "DeleteBeforeReplace/false",
			give: deploytest.ResourceOptions{
				DeleteBeforeReplace: &falseValue,
			},
			want: plugin.ConstructOptions{
				DeleteBeforeReplace: &falseValue,
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
				Protect: &trueValue,
			},
			want: plugin.ConstructOptions{
				Protect: &trueValue,
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
				RetainOnDelete: &trueValue,
			},
			want: plugin.ConstructOptions{
				RetainOnDelete: &trueValue,
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
				_, err := resmon.RegisterResource("pkgA:m:typA", "resA", false, give)
				require.NoError(t, err, "register resource")
				return nil
			}
			pluginCtx, err := newTestPluginContext(t, program)
			require.NoError(t, err, "build plugin context")

			evalSource := NewEvalSource(pluginCtx, runInfo, nil, EvalSourceOptions{})
			defer func() {
				assert.NoError(t, evalSource.Close(), "close eval source")
			}()

			var got plugin.ConstructOptions
			provider := &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// To keep test cases above simple,
					// nil out properties that are empty when unset.
					nilIfEmpty(&req.Options.Aliases)
					nilIfEmpty(&req.Options.Dependencies)
					nilIfEmpty(&req.Options.PropertyDependencies)
					nilIfEmpty(&req.Options.Providers)

					got = req.Options
					return plugin.ConstructResponse{
						URN: newURN(req.Type, req.Name, req.Parent),
					}, nil
				},
			}

			ctx := context.Background()
			iter, res := evalSource.Iterate(ctx, &testProviderSource{defaultProvider: provider})
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
							URN:          newURN(goal.Type, goal.Name, goal.Parent),
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
//      ProjectRoot: "/",
// 		Pwd:         "/",
// 		Program:     ".",
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
// 			urn, id := newURN(goal.Type, goal.Name, goal.Parent), resource.ID("id")

// 			assert.True(t, providers.IsProviderType(goal.Type))
// 			// The name of the provider resource is derived from the version requested.
// 			assert.Equal(t, "default_0_18_0", goal.Name)
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
// 		_, err := resmon.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
// 			resource.PropertyMap{}, nil, false, "0.18.1", nil)
// 		assert.NoError(t, err)

// 		// Re-uses pkgA's already-instantiated provider.
// 		_, err = resmon.RegisterResource("pkgA:m:typA", "resB", true, "", false, nil, "",
// 			resource.PropertyMap{}, nil, false, "0.18.1", nil)
// 		assert.NoError(t, err)

// 		// Triggers pkgA, v0.18.2
// 		_, err = resmon.RegisterResource("pkgA:m:typA", "resB", true, "", false, nil, "",
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
// 			urn, id := newURN(goal.Type, goal.Name, goal.Parent), resource.ID("id")

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

	sptr := func(s string) *string {
		return &s
	}

	bptr := func(b bool) *bool {
		return &b
	}

	makeAlias := func(parent *string, noParent *bool, name string) *pulumirpc.Alias {
		spec := &pulumirpc.Alias_Spec{
			Name: name,
		}
		if parent != nil {
			spec.Parent = &pulumirpc.Alias_Spec_ParentUrn{ParentUrn: *parent}
		}
		if noParent != nil {
			spec.Parent = &pulumirpc.Alias_Spec_NoParent{NoParent: *noParent}
		}

		return &pulumirpc.Alias{
			Alias: &pulumirpc.Alias_Spec_{
				Spec: spec,
			},
		}
	}

	tests := []struct {
		name     string
		input    *pulumirpc.Alias
		expected *pulumirpc.Alias
	}{
		{
			name:     `{Parent: "", NoParent: true} (transformed)`,
			input:    makeAlias(nil, bptr(true), ""),
			expected: makeAlias(nil, nil, ""),
		},
		{
			name:     `{Parent: "", NoParent: false} (transformed)`,
			input:    makeAlias(sptr(""), nil, ""),
			expected: makeAlias(nil, bptr(true), ""),
		},
		{
			name:     `{Parent: "", NoParent: false, Name: "name"} (transformed)`,
			input:    makeAlias(sptr(""), nil, "name"),
			expected: makeAlias(nil, bptr(true), "name"),
		},
		{
			name:     `{Parent: "", NoParent: true, Name: "name"} (transformed)`,
			input:    makeAlias(nil, bptr(true), "name"),
			expected: makeAlias(nil, nil, "name"),
		},
		{
			name:     `{Parent: "foo", NoParent: false} (no transform)`,
			input:    makeAlias(sptr("foo"), nil, ""),
			expected: makeAlias(sptr("foo"), nil, ""),
		},
		{
			name:     `{Parent: "foo", NoParent: false, Name: "name"} (no transform)`,
			input:    makeAlias(sptr("foo"), nil, "name"),
			expected: makeAlias(sptr("foo"), nil, "name"),
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

type providerSourceMock struct {
	Provider plugin.Provider
}

func (ps *providerSourceMock) GetProvider(ref providers.Reference) (plugin.Provider, bool) {
	return ps.Provider, ps.Provider != nil
}

var _ ProviderSource = (*providerSourceMock)(nil)

type decrypterMock struct {
	DecryptValueF func(
		ctx context.Context, ciphertext string) (string, error)
	BatchDecryptF func(
		ctx context.Context, ciphertexts []string) ([]string, error)
}

var _ config.Decrypter = (*decrypterMock)(nil)

func (d *decrypterMock) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if d.DecryptValueF != nil {
		return d.DecryptValueF(ctx, ciphertext)
	}
	panic("unimplemented")
}

func (d *decrypterMock) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	if d.BatchDecryptF != nil {
		return d.BatchDecryptF(ctx, ciphertexts)
	}
	panic("unimplemented")
}

func TestEvalSource(t *testing.T) {
	t.Parallel()

	t.Run("Stack", func(t *testing.T) {
		t.Parallel()
		src := &evalSource{
			runinfo: &EvalRunInfo{
				Target: &Target{
					Name: tokens.MustParseStackName("target-name"),
				},
			},
		}
		assert.Equal(t, tokens.MustParseStackName("target-name"), src.Stack())
	})
	t.Run("Iterate", func(t *testing.T) {
		t.Parallel()
		t.Run("config decrypt value error", func(t *testing.T) {
			t.Parallel()
			var decrypterCalled bool
			src := &evalSource{
				plugctx: &plugin.Context{
					Diag: &deploytest.NoopSink{},
				},

				runinfo: &EvalRunInfo{
					ProjectRoot: "/",
					Pwd:         "/",
					Program:     ".",
					Proj:        &workspace.Project{Name: "proj"},
					Target: &Target{
						Name: tokens.MustParseStackName("target-name"),
						Config: config.Map{
							config.MustMakeKey("test", "secret"): config.NewSecureValue("secret"),
						},
						Decrypter: &decrypterMock{
							DecryptValueF: func(ctx context.Context, ciphertext string) (string, error) {
								decrypterCalled = true
								return "", errors.New("expected fail")
							},
						},
					},
				},
			}
			_, err := src.Iterate(context.Background(), &providerSourceMock{})
			assert.ErrorContains(t, err, "failed to decrypt config")
			assert.True(t, decrypterCalled)
		})
		t.Run("failed to convert config to map", func(t *testing.T) {
			t.Parallel()

			var called int
			var decrypterCalled bool
			src := &evalSource{
				plugctx: &plugin.Context{
					Diag: &deploytest.NoopSink{},
				},
				runinfo: &EvalRunInfo{
					ProjectRoot: "/",
					Pwd:         "/",
					Program:     ".",
					Target: &Target{
						Config: config.Map{
							config.MustMakeKey("test", "secret"): config.NewSecureValue("secret"),
						},
						Decrypter: &decrypterMock{
							DecryptValueF: func(ctx context.Context, ciphertext string) (string, error) {
								decrypterCalled = true
								if called == 0 {
									// Will cause the next invocation to fail.
									called++
									return "", nil
								}
								return "", errors.New("expected fail")
							},
						},
					},
				},
			}
			_, err := src.Iterate(context.Background(), &providerSourceMock{})
			assert.ErrorContains(t, err, "failed to convert config to map")
			assert.True(t, decrypterCalled)
		})
	})
}

func TestResmonCancel(t *testing.T) {
	t.Parallel()
	done := make(chan error)
	rm := &resmon{
		cancel: make(chan bool, 10),
		done:   done,
	}
	err := errors.New("my error")

	go func() {
		// This ensures that cancel doesn't hang.
		done <- err
	}()

	// Cancel always returns nil or a joinErrors.
	assert.Equal(t, errors.Join(err), rm.Cancel())
}

func TestSourceEvalServeOptions(t *testing.T) {
	t.Parallel()
	assert.Len(t,
		sourceEvalServeOptions(nil, opentracing.SpanFromContext(context.Background()), "" /* logFile */),
		2,
	)

	assert.Len(t,
		sourceEvalServeOptions(&plugin.Context{
			DebugTraceMutex: &sync.Mutex{},
		}, opentracing.SpanFromContext(context.Background()), "logFile.log"),
		4,
	)
}

func TestEvalSourceIterator(t *testing.T) {
	t.Parallel()
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		var called bool
		iter := &evalSourceIterator{
			mon: &mockResmon{
				CancelF: func() error {
					called = true
					return nil
				},
			},
		}
		iter.Close()
		assert.True(t, called)
	})
	t.Run("ResourceMonitor", func(t *testing.T) {
		t.Parallel()
		var called bool
		mon := &mockResmon{
			CancelF: func() error { called = true; return nil },
		}
		iter := &evalSourceIterator{
			mon: mon,
		}
		iter.Close()
		assert.Equal(t, mon, iter.ResourceMonitor())
		assert.True(t, called)
	})
	t.Run("Next", func(t *testing.T) {
		t.Parallel()
		t.Run("iter.done", func(t *testing.T) {
			t.Parallel()
			iter := &evalSourceIterator{
				done: true,
			}
			evt, err := iter.Next()
			assert.Nil(t, evt)
			assert.NoError(t, err)
		})
	})
	t.Run("Abort", func(t *testing.T) {
		t.Parallel()
		abortChan := make(chan bool)
		iter := &evalSourceIterator{
			mon: &resmon{
				abortChan: abortChan,
			},
		}
		go func() {
			abortChan <- true
		}()
		evt, err := iter.Next()
		assert.ErrorContains(t, err, "EvalSourceIterator aborted")
		assert.Nil(t, evt)

		evt, err = iter.Next()
		assert.ErrorContains(t, err, "EvalSourceIterator aborted")
		assert.Nil(t, evt)
	})
}

func TestParseSourcePosition(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       *pulumirpc.SourcePosition
		expected    string
		errContains string
	}{
		{
			name:        "NilInput",
			input:       nil,
			expected:    "",
			errContains: "",
		},
		{
			name:        "InvalidLine",
			input:       &pulumirpc.SourcePosition{Line: 0},
			expected:    "",
			errContains: "invalid line number 0",
		},
		{
			name:        "InvalidColumn",
			input:       &pulumirpc.SourcePosition{Line: 1, Column: -1},
			expected:    "",
			errContains: "invalid column number -1",
		},
		{
			name:        "InvalidURI",
			input:       &pulumirpc.SourcePosition{Line: 1, Column: 1, Uri: ":invalid-uri:"},
			expected:    "",
			errContains: `parse ":invalid-uri:": missing protocol scheme`,
		},
		{
			name:        "UnrecognizedScheme",
			input:       &pulumirpc.SourcePosition{Line: 1, Column: 1, Uri: "http://example.com/file.txt"},
			expected:    "",
			errContains: "unrecognized scheme \"http\"",
		},
		{
			name:        "NonAbsolutePath",
			input:       &pulumirpc.SourcePosition{Line: 1, Column: 1, Uri: "file:relative/path/file.txt"},
			expected:    "",
			errContains: "source positions must include absolute paths",
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &sourcePositions{
				projectRoot: "/absolute/path/",
			}
			result, err := s.parseSourcePosition(tt.input)

			assert.Equal(t, tt.expected, result)
			if tt.errContains != "" {
				assert.ErrorContains(t, err, tt.errContains, result)
				assert.Equal(t, tt.expected, result)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type configSourceMock struct {
	GetPackageConfigF func(pkg tokens.Package) (resource.PropertyMap, error)
}

var _ plugin.ConfigSource = (*configSourceMock)(nil)

func (c *configSourceMock) GetPackageConfig(pkg tokens.Package) (resource.PropertyMap, error) {
	if c.GetPackageConfigF != nil {
		return c.GetPackageConfigF(pkg)
	}
	panic("unimplemented")
}

func TestDefaultProviders(t *testing.T) {
	t.Parallel()
	t.Run("normalizeProviderRequest", func(t *testing.T) {
		t.Parallel()
		t.Run("use defaultProvider", func(t *testing.T) {
			t.Parallel()
			v1 := semver.MustParse("0.1.0")
			d := &defaultProviders{
				defaultProviderInfo: map[tokens.Package]workspace.PackageDescriptor{
					tokens.Package("pkg"): {
						PluginSpec: workspace.PluginSpec{
							Version:           &v1,
							PluginDownloadURL: "github://owner/repo",
							Checksums:         map[string][]byte{"key": []byte("expected-checksum-value")},
						},
					},
				},
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return resource.PropertyMap{}, nil
					},
				},
			}
			req := d.normalizeProviderRequest(providers.NewProviderRequest(tokens.Package("pkg"), nil, "", nil, nil))
			assert.NotNil(t, req)
			assert.Equal(t, &v1, req.Version())
			assert.Equal(t, "github://owner/repo", req.PluginDownloadURL())
			assert.Equal(t, map[string][]byte{"key": []byte("expected-checksum-value")}, req.PluginChecksums())
		})
	})
	t.Run("newRegisterDefaultProviderEvent", func(t *testing.T) {
		t.Parallel()
		t.Run("error in GetPackageConfig()", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, expectedErr
					},
				},
			}
			_, _, err := d.newRegisterDefaultProviderEvent(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
	})
	t.Run("handleRequest", func(t *testing.T) {
		t.Parallel()
		t.Run("error in shouldDenyRequest", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, expectedErr
					},
				},
			}
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("error in newRegisterDefaultProviderEvent", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						if pkg == "pulumi" {
							// Enables shouldDenyRequest(req) to succeed as it always calls using
							// "pulumi".
							return nil, nil
						}
						return nil, expectedErr
					},
				},
			}
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("error due to cancel before registration", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			d := &defaultProviders{
				cancel: cancel,
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			}
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, context.Canceled)
		})
		t.Run("error cancel after registration, but before registration result", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)

			providerRegChan := make(chan *registerResourceEvent, 1)
			d := &defaultProviders{
				cancel:          cancel,
				providerRegChan: providerRegChan,
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			}
			go func() {
				// Cancel after reading the registration.
				<-providerRegChan
				cancel <- true
			}()
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, context.Canceled)
		})
	})
	t.Run("shouldDenyRequest", func(t *testing.T) {
		t.Parallel()
		t.Run("GetPackageConfigErr", func(t *testing.T) {
			t.Parallel()

			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, expectedErr
					},
				},
			}
			_, err := d.shouldDenyRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("disable-default-providers", func(t *testing.T) {
			t.Parallel()
			t.Run("invalid value", func(t *testing.T) {
				t.Parallel()
				d := &defaultProviders{
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
							return resource.PropertyMap{
								"disable-default-providers": resource.NewNumberProperty(100),
							}, nil
						},
					},
				}
				_, err := d.shouldDenyRequest(providers.ProviderRequest{})
				assert.ErrorContains(t, err, "Unexpected encoding of pulumi:disable-default-providers")
			})
			t.Run("empty value", func(t *testing.T) {
				t.Parallel()
				d := &defaultProviders{
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
							return resource.PropertyMap{
								"disable-default-providers": resource.NewStringProperty(""),
							}, nil
						},
					},
				}
				res, err := d.shouldDenyRequest(providers.ProviderRequest{})
				assert.NoError(t, err)
				assert.False(t, res)
			})
			t.Run("invalid list", func(t *testing.T) {
				t.Run("bad json", func(t *testing.T) {
					t.Parallel()
					d := &defaultProviders{
						config: &configSourceMock{
							GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
								return resource.PropertyMap{
									"disable-default-providers": resource.NewStringProperty("[[["),
								}, nil
							},
						},
					}
					res, err := d.shouldDenyRequest(providers.ProviderRequest{})
					assert.ErrorContains(t, err, "Failed to parse [[[")
					assert.True(t, res)
				})
				t.Run("mixed list values", func(t *testing.T) {
					t.Parallel()
					d := &defaultProviders{
						config: &configSourceMock{
							GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
								return resource.PropertyMap{
									"disable-default-providers": resource.NewStringProperty(`["foo", 2, 3]`),
								}, nil
							},
						},
					}
					res, err := d.shouldDenyRequest(providers.ProviderRequest{})
					assert.ErrorContains(t, err, "must be a string")
					assert.True(t, res)
				})
			})
		})
	})
	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()
		t.Run("serve respects cancel", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			d := &defaultProviders{
				cancel: cancel,
			}
			d.serve()
		})
		t.Run("getDefaultProviderRef respects cancel", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			d := &defaultProviders{
				cancel: cancel,
			}
			_, err := d.getDefaultProviderRef(providers.ProviderRequest{})
			assert.ErrorIs(t, err, context.Canceled)
		})
	})
}

func TestParseProviderRequest(t *testing.T) {
	t.Parallel()
	t.Run("bad version", func(t *testing.T) {
		t.Parallel()
		_, err := parseProviderRequest("", "bad-version", "", nil, nil)
		assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
	})
}

func TestInvoke(t *testing.T) {
	t.Parallel()
	t.Run("bad version", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{}
		_, err := rm.Invoke(context.Background(), &pulumirpc.ResourceInvokeRequest{
			Tok:     "pkgA:index:func",
			Version: "bad-version",
		})
		assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
	})
	t.Run("error in invoke", func(t *testing.T) {
		t.Parallel()

		plugctx, err := plugin.NewContext(context.Background(),
			&deploytest.NoopSink{}, &deploytest.NoopSink{},
			deploytest.NewPluginHostF(nil, nil, nil)(),
			nil, "", nil, false, nil)
		require.NoError(t, err)

		providerRegChan := make(chan *registerResourceEvent, 1)
		var called bool
		expectedErr := errors.New("expected error")

		mon, err := newResourceMonitor(&evalSource{
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "proj"},
				Target: &Target{
					Name: tokens.MustParseStackName("stack"),
				},
			},
			plugctx: plugctx,
		}, &providerSourceMock{
			Provider: &deploytest.Provider{
				InvokeF: func(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					called = true
					return plugin.InvokeResponse{}, expectedErr
				},
			},
		}, providerRegChan, nil, nil, nil, nil, opentracing.SpanFromContext(context.Background()))
		require.NoError(t, err)

		wg := &sync.WaitGroup{}
		wg.Add(1)
		// Needed so defaultProviders.handleRequest() doesn't hang.
		go func() {
			evt := <-providerRegChan
			evt.done <- &RegisterResult{
				State: &resource.State{
					ID:  "b2562429-e255-4b8f-904b-2bd239301ff2",
					URN: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				},
			}
			wg.Done()
		}()

		_, err = mon.Invoke(context.Background(), &pulumirpc.ResourceInvokeRequest{
			Tok:     "pkgA:index:func",
			Version: "1.0.0",
		})
		assert.ErrorContains(t, err, "returned an error")
		// Ensure the channel is read from.
		wg.Wait()
		assert.True(t, called)
	})
	t.Run("error in invoke", func(t *testing.T) {
		t.Parallel()

		plugctx, err := plugin.NewContext(context.Background(),
			&deploytest.NoopSink{}, &deploytest.NoopSink{},
			deploytest.NewPluginHostF(nil, nil, nil)(),
			nil, "", nil, false, nil)
		require.NoError(t, err)

		providerRegChan := make(chan *registerResourceEvent, 1)
		var called bool

		mon, err := newResourceMonitor(&evalSource{
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "proj"},
				Target: &Target{
					Name: tokens.MustParseStackName("stack"),
				},
			},
			plugctx: plugctx,
		}, &providerSourceMock{
			Provider: &deploytest.Provider{
				InvokeF: func(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					called = true
					return plugin.InvokeResponse{
						Failures: []plugin.CheckFailure{
							{
								Property: "some-property",
								Reason:   "expect failure",
							},
						},
					}, nil
				},
			},
		}, providerRegChan, nil, nil, nil, nil, opentracing.SpanFromContext(context.Background()))
		require.NoError(t, err)

		wg := &sync.WaitGroup{}
		wg.Add(1)
		// Needed so defaultProviders.handleRequest() doesn't hang.
		go func() {
			evt := <-providerRegChan
			evt.done <- &RegisterResult{
				State: &resource.State{
					ID:  "b2562429-e255-4b8f-904b-2bd239301ff2",
					URN: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				},
			}
			wg.Done()
		}()

		res, err := mon.Invoke(context.Background(), &pulumirpc.ResourceInvokeRequest{
			Tok:     "pkgA:index:func",
			Version: "1.0.0",
		})
		assert.NoError(t, err)
		assert.Equal(t, "some-property", res.Failures[0].Property)
		assert.Equal(t, "expect failure", res.Failures[0].Reason)
		// Ensure the channel is read from.
		wg.Wait()
		assert.True(t, called)
	})
}

func TestCall(t *testing.T) {
	t.Parallel()
	t.Run("bad version", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{}
		_, err := rm.Call(context.Background(), &pulumirpc.ResourceCallRequest{
			Tok:     "pkgA:index:func",
			Version: "bad-version",
		})
		assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
	})
	t.Run("error in call", func(t *testing.T) {
		t.Parallel()

		plugctx, err := plugin.NewContext(context.Background(),
			&deploytest.NoopSink{}, &deploytest.NoopSink{},
			deploytest.NewPluginHostF(nil, nil, nil)(),
			nil, "", nil, false, nil)
		require.NoError(t, err)

		providerRegChan := make(chan *registerResourceEvent, 1)
		var called bool
		expectedErr := errors.New("expected error")

		mon, err := newResourceMonitor(&evalSource{
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "proj"},
				Target: &Target{
					Name: tokens.MustParseStackName("stack"),
				},
			},
			plugctx: plugctx,
		}, &providerSourceMock{
			Provider: &deploytest.Provider{
				CallF: func(context.Context, plugin.CallRequest, *deploytest.ResourceMonitor) (plugin.CallResponse, error) {
					called = true
					return plugin.CallResponse{}, expectedErr
				},
			},
		}, providerRegChan, nil, nil, nil, nil, opentracing.SpanFromContext(context.Background()))
		require.NoError(t, err)

		abortChan := make(chan bool)
		cancel := make(chan bool)
		mon.abortChan = abortChan
		mon.cancel = cancel

		wg := &sync.WaitGroup{}
		wg.Add(1)
		// Needed so defaultProviders.handleRequest() doesn't hang.
		go func() {
			evt := <-providerRegChan
			evt.done <- &RegisterResult{
				State: &resource.State{
					ID:  "b2562429-e255-4b8f-904b-2bd239301ff2",
					URN: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				},
			}
			wg.Done()
		}()

		go func() {
			// the resource monitor should send a true value to the abort channel to indicate that the
			// iterator should shut down.
			val, ok := <-abortChan
			assert.True(t, ok)
			assert.True(t, val)
			close(cancel)
		}()

		_, err = mon.Call(context.Background(), &pulumirpc.ResourceCallRequest{
			Tok:     "pkgA:index:func",
			Version: "1.0.0",
		})
		assert.ErrorContains(t, err, "returned an error")
		// Ensure the channel is read from.
		wg.Wait()
		assert.True(t, called)
	})
	t.Run("handles args and arg dependencies", func(t *testing.T) {
		t.Parallel()

		plugctx, err := plugin.NewContext(context.Background(),
			&deploytest.NoopSink{}, &deploytest.NoopSink{},
			deploytest.NewPluginHostF(nil, nil, nil)(),
			nil, "", nil, false, nil)
		require.NoError(t, err)

		providerRegChan := make(chan *registerResourceEvent, 1)
		wg := &sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		// Needed so defaultProviders.handleRequest() doesn't hang.
		go func() {
			evt := <-providerRegChan
			evt.done <- &RegisterResult{
				State: &resource.State{
					ID:  "b2562429-e255-4b8f-904b-2bd239301ff2",
					URN: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				},
			}
			wg.Done()
		}()
		var called bool
		expectedErr := errors.New("expected error")

		mon, err := newResourceMonitor(&evalSource{
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "proj"},
				Target: &Target{
					Name: tokens.MustParseStackName("stack"),
				},
			},
			plugctx: plugctx,
		}, &providerSourceMock{
			Provider: &deploytest.Provider{
				CallF: func(
					_ context.Context,
					req plugin.CallRequest,
					_ *deploytest.ResourceMonitor,
				) (plugin.CallResponse, error) {
					assert.Equal(t,
						resource.PropertyMap{
							"test": resource.NewStringProperty("test-value"),
						},
						req.Args)
					require.Equal(t, 1, len(req.Options.ArgDependencies))
					assert.ElementsMatch(t,
						[]resource.URN{
							"urn:pulumi:stack::project::type::dep1",
							"urn:pulumi:stack::project::type::dep2",
							"urn:pulumi:stack::project::type::dep3",
						},
						req.Options.ArgDependencies["test"])
					called = true
					return plugin.CallResponse{}, expectedErr
				},
			},
		}, providerRegChan, nil, nil, nil, nil, opentracing.SpanFromContext(context.Background()))
		require.NoError(t, err)

		abortChan := make(chan bool)
		cancel := make(chan bool)
		mon.abortChan = abortChan
		mon.cancel = cancel

		go func() {
			// the resource monitor should send a true value to the abort channel to indicate that the
			// iterator should shut down.
			val, ok := <-abortChan
			assert.True(t, ok)
			assert.True(t, val)
			close(cancel)
		}()

		args, err := plugin.MarshalProperties(resource.PropertyMap{
			"test": resource.NewStringProperty("test-value"),
		}, plugin.MarshalOptions{})
		require.NoError(t, err)

		_, err = mon.Call(context.Background(), &pulumirpc.ResourceCallRequest{
			Tok:     "pkgA:index:func",
			Version: "1.0.0",
			Args:    args,
			ArgDependencies: map[string]*pulumirpc.ResourceCallRequest_ArgumentDependencies{
				"test": {
					Urns: []string{
						"urn:pulumi:stack::project::type::dep1",
						"urn:pulumi:stack::project::type::dep2",
						"urn:pulumi:stack::project::type::dep3",
					},
				},
			},
		})
		assert.ErrorContains(t, err, "returned an error")
		// Ensure the channel is read from.
		assert.True(t, called)
	})
	t.Run("catch invalid arg dependencies", func(t *testing.T) {
		t.Parallel()

		plugctx, err := plugin.NewContext(context.Background(),
			&deploytest.NoopSink{}, &deploytest.NoopSink{},
			deploytest.NewPluginHostF(nil, nil, nil)(),
			nil, "", nil, false, nil)
		require.NoError(t, err)

		providerRegChan := make(chan *registerResourceEvent, 1)
		wg := &sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		// Needed so defaultProviders.handleRequest() doesn't hang.
		go func() {
			evt := <-providerRegChan
			evt.done <- &RegisterResult{
				State: &resource.State{
					ID:  "b2562429-e255-4b8f-904b-2bd239301ff2",
					URN: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				},
			}
			wg.Done()
		}()

		mon, err := newResourceMonitor(&evalSource{
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "proj"},
				Target: &Target{
					Name: tokens.MustParseStackName("stack"),
				},
			},
			plugctx: plugctx,
		}, &providerSourceMock{
			Provider: &deploytest.Provider{
				CallF: func(context.Context, plugin.CallRequest, *deploytest.ResourceMonitor) (plugin.CallResponse, error) {
					assert.Fail(t, "Call should not be called")
					return plugin.CallResponse{}, nil
				},
			},
		}, providerRegChan, nil, nil, nil, nil, opentracing.SpanFromContext(context.Background()))
		require.NoError(t, err)

		args, err := plugin.MarshalProperties(resource.PropertyMap{
			"test": resource.NewStringProperty("test-value"),
		}, plugin.MarshalOptions{})
		require.NoError(t, err)

		_, err = mon.Call(context.Background(), &pulumirpc.ResourceCallRequest{
			Tok:     "pkgA:index:func",
			Version: "1.0.0",
			Args:    args,
			ArgDependencies: map[string]*pulumirpc.ResourceCallRequest_ArgumentDependencies{
				"test": {
					Urns: []string{
						"invalid urn",
					},
				},
			},
		})
		assert.ErrorContains(t, err, "invalid dependency")
	})
	t.Run("catch invalid arg dependencies", func(t *testing.T) {
		t.Parallel()

		plugctx, err := plugin.NewContext(context.Background(),
			&deploytest.NoopSink{}, &deploytest.NoopSink{},
			deploytest.NewPluginHostF(nil, nil, nil)(),
			nil, "", nil, false, nil)
		require.NoError(t, err)

		providerRegChan := make(chan *registerResourceEvent, 1)
		wg := &sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		// Needed so defaultProviders.handleRequest() doesn't hang.
		go func() {
			evt := <-providerRegChan
			evt.done <- &RegisterResult{
				State: &resource.State{
					ID:  "b2562429-e255-4b8f-904b-2bd239301ff2",
					URN: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				},
			}
			wg.Done()
		}()

		mon, err := newResourceMonitor(&evalSource{
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj:        &workspace.Project{Name: "proj"},
				Target: &Target{
					Name: tokens.MustParseStackName("stack"),
				},
			},
			plugctx: plugctx,
		}, &providerSourceMock{
			Provider: &deploytest.Provider{
				CallF: func(context.Context, plugin.CallRequest, *deploytest.ResourceMonitor) (plugin.CallResponse, error) {
					return plugin.CallResponse{
						Return: resource.PropertyMap{
							"result": resource.NewNumberProperty(100),
						},
						ReturnDependencies: map[resource.PropertyKey][]resource.URN{
							"prop": {
								"urn:pulumi:stack::project::type::dep1",
								"urn:pulumi:stack::project::type::dep2",
								"urn:pulumi:stack::project::type::dep3",
							},
						},
						Failures: []plugin.CheckFailure{
							{
								Property: "some-prop",
								Reason:   "expected failure",
							},
						},
					}, nil
				},
			},
		}, providerRegChan, nil, nil, nil, nil, opentracing.SpanFromContext(context.Background()))
		require.NoError(t, err)

		args, err := plugin.MarshalProperties(resource.PropertyMap{
			"test": resource.NewStringProperty("test-value"),
		}, plugin.MarshalOptions{})
		require.NoError(t, err)

		res, err := mon.Call(context.Background(), &pulumirpc.ResourceCallRequest{
			Tok:     "pkgA:index:func",
			Version: "1.0.0",
			Args:    args,
		})
		assert.NoError(t, err)
		assert.Equal(t,
			map[string]interface{}{
				"result": float64(100),
			}, res.Return.AsMap())
		assert.Equal(t,
			[]string{
				"urn:pulumi:stack::project::type::dep1",
				"urn:pulumi:stack::project::type::dep2",
				"urn:pulumi:stack::project::type::dep3",
			}, res.ReturnDependencies["prop"].Urns)
		assert.Equal(t, &pulumirpc.CheckFailure{
			Property: "some-prop",
			Reason:   "expected failure",
		}, res.Failures[0])
	})
}

func TestReadResource(t *testing.T) {
	t.Parallel()
	t.Run("bad parent", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{}
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:   "foo:bar:some-type",
			Parent: "invalid-parent",
		})
		assert.ErrorContains(t, err, "invalid parent URN")
	})
	t.Run("handles error from parseProviderRequest", func(t *testing.T) {
		t.Parallel()
		cancel := make(chan bool, 1)
		cancel <- true
		rm := &resmon{
			defaultProviders: &defaultProviders{
				cancel: cancel,
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
		}
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:    "foo:bar:some-type",
			Version: "1.0.0",
		})
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("handles invalid dependencies", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{
			defaultProviders: &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
		}
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:    "pulumi:providers:fake-provider",
			Version: "1.0.0",
			Dependencies: []string{
				"urn:pulumi:stack::project::type::dep1",
				"urn:pulumi:stack::project::type::dep2",
				"invalidURN",
			},
		})
		assert.ErrorContains(t, err, "invalid URN")
	})
	t.Run("handles invalid dependencies", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{
			defaultProviders: &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
		}
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:    "pulumi:providers:fake-provider",
			Version: "1.0.0",
			Dependencies: []string{
				"urn:pulumi:stack::project::type::dep1",
				"urn:pulumi:stack::project::type::dep2",
				"invalidURN",
			},
		})
		assert.ErrorContains(t, err, "invalid URN")
	})
	t.Run("handles additional secret outputs", func(t *testing.T) {
		t.Parallel()
		regReadChan := make(chan *readResourceEvent, 1)
		rm := &resmon{
			regReadChan: regReadChan,
			defaultProviders: &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			evt := <-regReadChan
			assert.Equal(t, []resource.PropertyKey{"foo"}, evt.additionalSecretOutputs)
			evt.done <- &ReadResult{
				State: &resource.State{},
			}
			wg.Done()
		}()
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:                    "pulumi:providers:fake-provider",
			Version:                 "1.0.0",
			AdditionalSecretOutputs: []string{"foo"},
		})
		assert.NoError(t, err)
		wg.Wait()
	})
	t.Run("resource monitor shut down while sending resource registration", func(t *testing.T) {
		t.Parallel()
		cancel := make(chan bool, 1)
		rm := &resmon{
			cancel: cancel,
			defaultProviders: &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			cancel <- true
			wg.Done()
		}()
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:    "pulumi:providers:fake-provider",
			Version: "1.0.0",
		})
		assert.ErrorContains(t, err, "resource monitor shut down while sending resource registration")
		wg.Wait()
	})
	t.Run("resource monitor shut down while waiting on step's done channel", func(t *testing.T) {
		t.Parallel()
		// requests := make(chan
		cancel := make(chan bool, 1)
		regReadChan := make(chan *readResourceEvent, 1)
		rm := &resmon{
			regReadChan: regReadChan,
			cancel:      cancel,
			defaultProviders: &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
		}
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			<-regReadChan
			cancel <- true
			wg.Done()
		}()
		_, err := rm.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
			Type:    "pulumi:providers:fake-provider",
			Version: "1.0.0",
		})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		wg.Wait()
	})
}

func TestRegisterResource(t *testing.T) {
	t.Parallel()
	t.Run("gracefully handle cancellation", func(t *testing.T) {
		t.Parallel()
		t.Run("resource monitor shut down while sending resource registration", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			rm := &resmon{
				cancel: cancel,
			}
			_, err := rm.RegisterResource(context.Background(), &pulumirpc.RegisterResourceRequest{})
			assert.ErrorContains(t, err, "resource monitor shut down while sending resource registration")
		})
		t.Run("resource monitor shut down while waiting on step's done channel", func(t *testing.T) {
			t.Parallel()
			regChan := make(chan *registerResourceEvent, 1)
			cancel := make(chan bool, 1)
			go func() {
				<-regChan
				cancel <- true
			}()

			rm := &resmon{
				regChan: regChan,
				cancel:  cancel,
			}
			_, err := rm.RegisterResource(context.Background(), &pulumirpc.RegisterResourceRequest{})
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		})
		t.Run("resource monitor shut down while waiting on step's done channel", func(t *testing.T) {
			t.Parallel()
			regChan := make(chan *registerResourceEvent, 1)
			cancel := make(chan bool, 1)
			go func() {
				<-regChan
				cancel <- true
			}()

			rm := &resmon{
				regChan: regChan,
				cancel:  cancel,
			}
			_, err := rm.RegisterResource(context.Background(), &pulumirpc.RegisterResourceRequest{})
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		})
	})
	t.Run("remote handles improper version", func(t *testing.T) {
		t.Parallel()
		regChan := make(chan *registerResourceEvent, 1)
		go func() {
			evt := <-regChan
			evt.done <- &RegisterResult{
				State: &resource.State{},
			}
		}()
		rm := &resmon{}
		req := &pulumirpc.RegisterResourceRequest{
			Type:    "foo:bar:some-type",
			Version: "improper-version",
			Remote:  true,
		}
		_, err := rm.RegisterResource(context.Background(), req)
		assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
	})
	t.Run("custom handles improper version", func(t *testing.T) {
		t.Parallel()
		regChan := make(chan *registerResourceEvent, 1)
		go func() {
			evt := <-regChan
			evt.done <- &RegisterResult{
				State: &resource.State{},
			}
		}()
		rm := &resmon{}
		req := &pulumirpc.RegisterResourceRequest{
			Type:    "foo:bar:some-type",
			Version: "improper-version",
			Custom:  true,
		}
		require.False(t, providers.IsProviderType(tokens.Type(req.GetType())))
		_, err := rm.RegisterResource(context.Background(), req)
		assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
	})
	t.Run("custom provider handles improper version", func(t *testing.T) {
		t.Parallel()
		regChan := make(chan *registerResourceEvent, 1)
		go func() {
			evt := <-regChan
			evt.done <- &RegisterResult{
				State: &resource.State{},
			}
		}()
		rm := &resmon{}
		req := &pulumirpc.RegisterResourceRequest{
			Type:    "pulumi:providers:some-type",
			Version: "improper-version",
			Custom:  true,
		}
		require.True(t, providers.IsProviderType(tokens.Type(req.GetType())))
		_, err := rm.RegisterResource(context.Background(), req)
		assert.ErrorContains(t, err, "passed invalid version")
	})
	t.Run("invalid alias URN", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{}
		req := &pulumirpc.RegisterResourceRequest{
			Type: "pulumi:providers:some-type",
			AliasURNs: []string{
				"invalid-urn",
			},
		}
		_, err := rm.RegisterResource(context.Background(), req)
		assert.ErrorContains(t, err, "invalid alias URN")
	})
	t.Run("invalid dependency on property", func(t *testing.T) {
		t.Parallel()
		rm := &resmon{
			defaultProviders: &defaultProviders{
				defaultProviderInfo: map[tokens.Package]workspace.PackageDescriptor{},
			},
		}
		req := &pulumirpc.RegisterResourceRequest{
			Type:    "pulumi:providers:some-type",
			Version: "1.0.0",
			PropertyDependencies: map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies{
				"invalid-urn": {
					Urns: []string{"bad-urn"},
				},
			},
		}
		_, err := rm.RegisterResource(context.Background(), req)
		assert.ErrorContains(t, err, "invalid dependency on property")
	})
	t.Run("remote resource", func(t *testing.T) {
		t.Parallel()
		t.Run("invalid provider in providers", func(t *testing.T) {
			t.Parallel()
			requests := make(chan defaultProviderRequest, 1)
			go func() {
				evt := <-requests
				ref, err := providers.NewReference(
					"urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
					"b2562429-e255-4b8f-904b-2bd239301ff2")
				require.NoError(t, err)
				evt.response <- defaultProviderResponse{
					ref: ref,
				}
			}()
			rm := &resmon{
				defaultProviders: &defaultProviders{
					requests: requests,
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
							return nil, nil
						},
					},
				},
			}
			req := &pulumirpc.RegisterResourceRequest{
				Version: "1.0.0",
				Type:    "pulumi:providers:some-type",
				Remote:  true,
				Providers: map[string]string{
					"name": "not-an-urn::id",
				},
			}
			_, err := rm.RegisterResource(context.Background(), req)
			assert.ErrorContains(t, err, "could not parse provider reference")
		})
		t.Run("catch denied default provider", func(t *testing.T) {
			t.Parallel()
			requests := make(chan defaultProviderRequest, 1)
			go func() {
				evt := <-requests
				ref, err := providers.NewReference(
					"urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
					"denydefaultprovider")
				require.NoError(t, err)
				evt.response <- defaultProviderResponse{
					ref: ref,
				}
			}()
			rm := &resmon{
				defaultProviders: &defaultProviders{
					requests: requests,
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
							return nil, nil
						},
					},
				},
				providers: &providerSourceMock{
					Provider: &deploytest.Provider{},
				},
			}
			req := &pulumirpc.RegisterResourceRequest{
				Version: "1.0.0",
				Type:    "pulumi:providers:some-type",
				Remote:  true,
				Providers: map[string]string{
					"missing": "urn:pulumi:stack::project::pulumi:providers:aws::prov-1::uuid",
				},
			}
			_, err := rm.RegisterResource(context.Background(), req)
			assert.ErrorContains(t, err,
				"Default provider for 'pulumi' disabled. 'pulumi:providers:some-type' must use an explicit provider.")
		})
		t.Run("unknown provider", func(t *testing.T) {
			t.Parallel()
			requests := make(chan defaultProviderRequest, 1)
			go func() {
				evt := <-requests
				ref, err := providers.NewReference(
					"urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
					"b2562429-e255-4b8f-904b-2bd239301ff2")
				require.NoError(t, err)
				evt.response <- defaultProviderResponse{
					ref: ref,
				}
			}()
			rm := &resmon{
				defaultProviders: &defaultProviders{
					requests: requests,
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
							return nil, nil
						},
					},
				},
				providers: &providerSourceMock{},
			}
			req := &pulumirpc.RegisterResourceRequest{
				Version: "1.0.0",
				Type:    "pulumi:providers:some-type",
				Remote:  true,
				Providers: map[string]string{
					"missing": "urn:pulumi:stack::project::pulumi:providers:aws::prov-1::uuid",
				},
			}
			_, err := rm.RegisterResource(context.Background(), req)
			assert.ErrorContains(t, err, "unknown provider")
		})
	})
	t.Run("output dependencies", func(t *testing.T) {
		t.Parallel()
		requests := make(chan defaultProviderRequest, 1)
		go func() {
			evt := <-requests
			ref, err := providers.NewReference(
				"urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				"b2562429-e255-4b8f-904b-2bd239301ff2")
			require.NoError(t, err)
			evt.response <- defaultProviderResponse{
				ref: ref,
			}
		}()
		rm := &resmon{
			defaultProviders: &defaultProviders{
				requests: requests,
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
			providers: &providerSourceMock{
				Provider: &deploytest.Provider{
					DialMonitorF: func(
						ctx context.Context, endpoint string,
					) (*deploytest.ResourceMonitor, error) {
						return nil, nil
					},
					ConstructF: func(
						context.Context,
						plugin.ConstructRequest,
						*deploytest.ResourceMonitor,
					) (plugin.ConstructResponse, error) {
						return plugin.ConstructResponse{
							OutputDependencies: map[resource.PropertyKey][]resource.URN{
								"expected-key-1": {
									"untrusted-urn-1",
								},
								"expected-key-2": {
									"untrusted-urn-1",
									"untrusted-urn-2",
								},
							},
						}, nil
					},
				},
			},
		}
		req := &pulumirpc.RegisterResourceRequest{
			Version: "1.0.0",
			Type:    "pulumi:providers:some-type",
			Remote:  true,
		}
		res, err := rm.RegisterResource(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, []string{
			"untrusted-urn-1",
		}, res.PropertyDependencies["expected-key-1"].Urns)
		assert.Equal(t, []string{
			"untrusted-urn-1",
			"untrusted-urn-2",
		}, res.PropertyDependencies["expected-key-2"].Urns)
	})
	t.Run("not remote resource", func(t *testing.T) {
		t.Parallel()
		t.Run("additional secret keys", func(t *testing.T) {
			t.Parallel()
			regChan := make(chan *registerResourceEvent, 1)
			go func() {
				evt := <-regChan
				evt.done <- &RegisterResult{
					State: &resource.State{},
				}
			}()
			rm := &resmon{
				regChan: regChan,
				componentProviders: map[resource.URN]map[string]string{
					"urn:pulumi:stack::project::type::foo": {
						"urn:pulumi:stack::project::type::prov1": "",
						"urn:pulumi:stack::project::type::prov2": "expected-value",
					},
				},
			}
			req := &pulumirpc.RegisterResourceRequest{
				Provider: "urn:pulumi:stack::project::type::bar",
				Parent:   "urn:pulumi:stack::project::type::foo",
				AdditionalSecretOutputs: []string{
					"a",
					"b",
					"c",
				},
			}
			_, err := rm.RegisterResource(context.Background(), req)
			assert.NoError(t, err)
			assert.Equal(t,
				[]string{"a", "b", "c"},
				req.AdditionalSecretOutputs)
		})
		t.Run("handle invalid custom timeouts", func(t *testing.T) {
			t.Parallel()
			t.Run("Create", func(t *testing.T) {
				t.Parallel()
				regChan := make(chan *registerResourceEvent, 1)
				go func() {
					evt := <-regChan
					evt.done <- &RegisterResult{
						State: &resource.State{},
					}
				}()
				rm := &resmon{
					regChan:            regChan,
					componentProviders: map[resource.URN]map[string]string{},
				}
				req := &pulumirpc.RegisterResourceRequest{
					CustomTimeouts: &pulumirpc.RegisterResourceRequest_CustomTimeouts{
						Create: "invalid",
					},
				}
				_, err := rm.RegisterResource(context.Background(), req)
				assert.ErrorContains(t, err, "unable to parse customTimeout Value")
			})
			t.Run("Delete", func(t *testing.T) {
				t.Parallel()
				regChan := make(chan *registerResourceEvent, 1)
				go func() {
					evt := <-regChan
					evt.done <- &RegisterResult{
						State: &resource.State{},
					}
				}()
				rm := &resmon{
					regChan:            regChan,
					componentProviders: map[resource.URN]map[string]string{},
				}
				req := &pulumirpc.RegisterResourceRequest{
					CustomTimeouts: &pulumirpc.RegisterResourceRequest_CustomTimeouts{
						Delete: "invalid",
					},
				}
				_, err := rm.RegisterResource(context.Background(), req)
				assert.ErrorContains(t, err, "unable to parse customTimeout Value")
			})
			t.Run("Update", func(t *testing.T) {
				t.Parallel()
				regChan := make(chan *registerResourceEvent, 1)
				go func() {
					evt := <-regChan
					evt.done <- &RegisterResult{
						State: &resource.State{},
					}
				}()
				rm := &resmon{
					regChan:            regChan,
					componentProviders: map[resource.URN]map[string]string{},
				}
				req := &pulumirpc.RegisterResourceRequest{
					CustomTimeouts: &pulumirpc.RegisterResourceRequest_CustomTimeouts{
						Update: "invalid",
					},
				}
				_, err := rm.RegisterResource(context.Background(), req)
				assert.ErrorContains(t, err, "unable to parse customTimeout Value")
			})
		})
	})
}

func TestValidationFailures(t *testing.T) {
	t.Parallel()

	s, _ := status.Newf(codes.InvalidArgument, "bad request").WithDetails(
		&pulumirpc.InputPropertiesError{
			Errors: []*pulumirpc.InputPropertiesError_PropertyError{
				{
					Reason:       "missing",
					PropertyPath: "testproperty",
				},
				{
					Reason:       "nested property error",
					PropertyPath: "nested[0]",
				},
			},
		},
	)
	badRequestError := s.Err()

	cases := []struct {
		name           string
		err            error
		expectedStderr string
	}{
		{
			name:           "regular error",
			err:            errors.New("test error"),
			expectedStderr: "error: pulumi:providers:some-type resource 'some-name' has a problem: test error\n",
		},
		{
			name: "bad request",
			err:  badRequestError,
			expectedStderr: "error: pulumi:providers:some-type resource 'some-name' has a problem: bad request\n" +
				"\t\t- property testproperty with value '{testvalue}' has a problem: missing\n" +
				"\t\t- property nested[0] with value '{nestedvalue}' has a problem: nested property error\n",
		},
	}
	for _, c := range cases {
		cancel := make(chan bool)
		abortChan := make(chan bool)
		go func() {
			// the resource monitor should send a true value to the abort channel to indicate that the
			// iterator should shut down.
			val, ok := <-abortChan
			assert.True(t, ok)
			assert.True(t, val)
			close(cancel)
		}()
		requests := make(chan defaultProviderRequest, 1)
		go func() {
			evt := <-requests
			ref, err := providers.NewReference(
				"urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0",
				"b2562429-e255-4b8f-904b-2bd239301ff2")
			require.NoError(t, err)
			evt.response <- defaultProviderResponse{
				ref: ref,
			}
		}()
		var stdout, stderr bytes.Buffer
		rm := &resmon{
			diagnostics: diagtest.MockSink(&stdout, &stderr),
			cancel:      cancel,
			abortChan:   abortChan,
			defaultProviders: &defaultProviders{
				requests: requests,
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (resource.PropertyMap, error) {
						return nil, nil
					},
				},
			},
			providers: &providerSourceMock{
				Provider: &deploytest.Provider{
					DialMonitorF: func(
						ctx context.Context, endpoint string,
					) (*deploytest.ResourceMonitor, error) {
						return nil, nil
					},
					ConstructF: func(ctx context.Context, req plugin.ConstructRequest, monitor *deploytest.ResourceMonitor,
					) (plugin.ConstructResult, error) {
						return plugin.ConstructResult{}, c.err
					},
				},
			},
		}

		props := resource.PropertyMap{
			"testproperty": resource.NewPropertyValue("testvalue"),
			"nested": resource.NewArrayProperty(
				[]resource.PropertyValue{resource.NewPropertyValue("nestedvalue")},
			),
		}

		marshalledProps, err := plugin.MarshalProperties(props, plugin.MarshalOptions{})
		assert.NoError(t, err)

		req := &pulumirpc.RegisterResourceRequest{
			Version: "1.0.0",
			Type:    "pulumi:providers:some-type",
			Name:    "some-name",
			Remote:  true,
			Object:  marshalledProps,
		}
		_, err = rm.RegisterResource(context.Background(), req)
		assert.ErrorContains(t, err, "resource monitor shut down")
		assert.Equal(t, c.expectedStderr, stderr.String())
		assert.Equal(t, "", stdout.String())
	}
}

func TestDowngradeOutputValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    resource.PropertyMap
		expected resource.PropertyMap
	}{
		{
			"plain",
			resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
				"bar": resource.NewNumberProperty(42),
			},
			resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
				"bar": resource.NewNumberProperty(42),
			},
		},
		{
			"secret",
			resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("hello")),
			},
			resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("hello")),
			},
		},
		{
			"output",
			resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("hello"),
					Known:   true,
				}),
			},
			resource.PropertyMap{
				"foo": resource.NewStringProperty("hello"),
			},
		},
		{
			"secret output",
			resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{
					Element: resource.NewStringProperty("hello"),
					Known:   true,
					Secret:  true,
				}),
			},
			resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("hello")),
			},
		},
		{
			"unknown output",
			resource.PropertyMap{
				"foo": resource.NewOutputProperty(resource.Output{}),
			},
			resource.PropertyMap{
				"foo": resource.MakeComputed(resource.NewStringProperty("")),
			},
		},
		{
			"unknown resource reference",
			resource.PropertyMap{
				"foo": resource.NewResourceReferenceProperty(resource.ResourceReference{
					URN: "urn:pulumi:stack::project::package:module:resource::name",
					ID:  resource.NewOutputProperty(resource.Output{}),
				}),
			},
			resource.PropertyMap{
				"foo": resource.NewResourceReferenceProperty(resource.ResourceReference{
					URN: "urn:pulumi:stack::project::package:module:resource::name",
					ID:  resource.MakeComputed(resource.NewStringProperty("")),
				}),
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := downgradeOutputValues(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
