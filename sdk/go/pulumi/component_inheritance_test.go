// Copyright 2026, Pulumi Corporation.
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

package pulumi

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// inheritanceBase stands in for a generated base component proxy: a struct that embeds ResourceState and
// declares its own output.
type inheritanceBase struct {
	ResourceState

	Out1 StringOutput `pulumi:"out1"`
}

// inheritanceDerived stands in for a generated derived component: it embeds the base proxy (transitively
// embedding ResourceState) and adds its own output.
type inheritanceDerived struct {
	inheritanceBase

	Out2 StringOutput `pulumi:"out2"`
}

// plainEmbed is an anonymous embed that does NOT embed ResourceState, so its fields must never be treated as
// resource outputs.
type plainEmbed struct {
	Ignored StringOutput `pulumi:"ignored"`
}

type withPlainEmbed struct {
	ResourceState
	plainEmbed

	Out StringOutput `pulumi:"out"`
}

type inheritanceChild struct {
	CustomResourceState
}

// constructBaseMonitor is a mock resource monitor that supports ConstructBaseResource, driven by a settable
// function. Implementing the interface also makes GetDeploymentInfo advertise CONSTRUCT_BASE support.
type constructBaseMonitor struct {
	testMonitor
	ConstructBaseResourceF func(args MockConstructBaseArgs) (resource.PropertyMap, error)
}

func (m *constructBaseMonitor) ConstructBaseResource(args MockConstructBaseArgs) (resource.PropertyMap, error) {
	if m.ConstructBaseResourceF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.ConstructBaseResourceF(args)
}

// TestMakeResourceStateEmbeddedRecursion verifies that makeResourceState recurses into anonymous embedded
// base proxies so outputs at every level of the embedding chain resolve through the normal register path.
func TestMakeResourceStateEmbeddedRecursion(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return args.Name + "_id", resource.PropertyMap{
				"out1": resource.NewProperty("base-value"),
				"out2": resource.NewProperty("derived-value"),
			}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		comp := &inheritanceDerived{}
		require.NoError(t, ctx.RegisterComponentResource("test:index:Derived", "d", comp))

		v1, known1, _, _, err := await(comp.Out1)
		require.NoError(t, err)
		assert.True(t, known1)
		assert.Equal(t, "base-value", v1)

		v2, known2, _, _, err := await(comp.Out2)
		require.NoError(t, err)
		assert.True(t, known2)
		assert.Equal(t, "derived-value", v2)

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

// TestMakeResourceStateIgnoresNonResourceEmbed verifies that an anonymous embed which does not carry resource
// state is left entirely alone: its fields are neither seeded nor resolved.
func TestMakeResourceStateIgnoresNonResourceEmbed(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return args.Name + "_id", resource.PropertyMap{
				"out":     resource.NewProperty("value"),
				"ignored": resource.NewProperty("should-not-resolve"),
			}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		comp := &withPlainEmbed{}
		require.NoError(t, ctx.RegisterComponentResource("test:index:WithPlain", "w", comp))

		v, _, _, _, err := await(comp.Out)
		require.NoError(t, err)
		assert.Equal(t, "value", v)

		// The plain embed's field was never wired as an output, so it stays the zero value.
		assert.Nil(t, internal.GetOutputState(comp.Ignored))

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

// newBaseConstructContext builds a context in base-construct (attach) mode, mirroring what the provider host
// does when serving a ConstructBase call: no root stack is registered, and the first component registration
// adopts adoptURN.
func newBaseConstructContext(t *testing.T, mocks MockResourceMonitor, adoptURN string) *Context {
	t.Helper()
	ctx, err := NewContext(t.Context(), RunInfo{
		Project:          "project",
		Stack:            "stack",
		Mocks:            mocks,
		baseConstructURN: adoptURN,
	})
	require.NoError(t, err)
	return ctx
}

// TestAttachModeAdoptsURN verifies that the first component registration on a base-mode context adopts the
// URN without a RegisterResource RPC, and that children parent to the adopted URN.
func TestAttachModeAdoptsURN(t *testing.T) {
	t.Parallel()

	adoptURN := "urn:pulumi:stack::project::test:index:Derived::d"

	var mu sync.Mutex
	var registered []string
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			mu.Lock()
			registered = append(registered, args.TypeToken)
			mu.Unlock()
			return args.Name + "_id", resource.PropertyMap{}, nil
		},
	}

	ctx := newBaseConstructContext(t, mocks, adoptURN)
	defer func() { _ = ctx.Close() }()

	comp := &inheritanceBase{}
	require.NoError(t, ctx.RegisterComponentResource("test:index:Base", "d", comp))

	child := &inheritanceChild{}
	require.NoError(t, ctx.RegisterResource("test:index:Child", "c", nil, child, Parent(comp)))

	require.NoError(t, ctx.wait())

	// The base component adopted the URN rather than registering a new resource.
	urn, _, _, err := comp.URN().awaitURN(t.Context())
	require.NoError(t, err)
	assert.Equal(t, adoptURN, string(urn))

	// The child parents to the adopted URN.
	childURN, _, _, err := child.URN().awaitURN(t.Context())
	require.NoError(t, err)
	assert.Contains(t, string(childURN), "test:index:Derived$test:index:Child::c")

	// Only the child hit the monitor; the base was adopted, not registered.
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"test:index:Child"}, registered)
}

// TestAttachModeSuppressesRegisterOutputs verifies that RegisterResourceOutputs is a no-op for the adopted
// resource, since the most-derived registration owns finalizing its outputs.
func TestAttachModeSuppressesRegisterOutputs(t *testing.T) {
	t.Parallel()

	adoptURN := "urn:pulumi:stack::project::test:index:Derived::d"

	var registeredOutputs atomic.Bool
	mocks := &testMonitor{
		RegisterResourceOutputsF: func() (*emptypb.Empty, error) {
			registeredOutputs.Store(true)
			return &emptypb.Empty{}, nil
		},
	}

	ctx := newBaseConstructContext(t, mocks, adoptURN)
	defer func() { _ = ctx.Close() }()

	comp := &inheritanceBase{}
	require.NoError(t, ctx.RegisterComponentResource("test:index:Base", "d", comp))
	require.NoError(t, ctx.RegisterResourceOutputs(comp, Map{"out": String("x")}))

	require.NoError(t, ctx.wait())

	assert.False(t, registeredOutputs.Load(), "RegisterResourceOutputs must be suppressed for the adopted resource")
}

// TestConstructBaseHost exercises the provider-host entry point end to end: the base construct adopts the
// request's URN (no RegisterResource RPC, so no monitor is needed), and the returned state carries the
// flattened outputs collected across the embedded base proxy.
func TestConstructBaseHost(t *testing.T) {
	t.Parallel()

	adoptURN := "urn:pulumi:mystack::myproject::test:index:Derived::d"
	req := &pulumirpc.ConstructBaseRequest{
		Project:         "myproject",
		Stack:           "mystack",
		Type:            "test:index:Base",
		Name:            "d",
		Urn:             adoptURN,
		MostDerivedType: "test:index:Derived",
	}

	resp, err := constructBase(t.Context(), req, nil, func(
		ctx *Context, typ, name string, inputs map[string]any, opts ResourceOption,
	) (URNInput, Input, error) {
		comp := &inheritanceDerived{}
		require.NoError(t, ctx.RegisterComponentResource(typ, name, comp, opts))
		comp.Out1 = String("base-value").ToStringOutput()
		comp.Out2 = String("derived-value").ToStringOutput()
		return newConstructResult(comp)
	})
	require.NoError(t, err)

	state, err := plugin.UnmarshalProperties(resp.GetState(), plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	require.NoError(t, err)
	// The flattened state carries outputs from both the embedded base and the derived component.
	assert.Equal(t, "base-value", state["out1"].StringValue())
	assert.Equal(t, "derived-value", state["out2"].StringValue())
}

// TestConstructBaseResolvesEmbeddedOutputs verifies that ConstructBase resolves the base's returned outputs
// onto the embedded proxy's fields while leaving the deriving component's own outputs untouched.
func TestConstructBaseResolvesEmbeddedOutputs(t *testing.T) {
	t.Parallel()

	mocks := &constructBaseMonitor{
		ConstructBaseResourceF: func(args MockConstructBaseArgs) (resource.PropertyMap, error) {
			assert.Equal(t, "test:index:Base", args.BaseType)
			return resource.PropertyMap{
				"out1": resource.NewProperty("base-out"),
			}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		comp := &inheritanceDerived{}
		require.NoError(t, ctx.RegisterComponentResource("test:index:Derived", "d", comp))

		// The deriving body sets its own output directly.
		comp.Out2 = String("derived-out").ToStringOutput()

		require.NoError(t, ctx.ConstructBase(comp, "test:index:Base", nil, "" /*packageRef*/))

		v1, known1, _, _, err := await(comp.Out1)
		require.NoError(t, err)
		assert.True(t, known1)
		assert.Equal(t, "base-out", v1)

		v2, _, _, _, err := await(comp.Out2)
		require.NoError(t, err)
		assert.Equal(t, "derived-out", v2)

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

// TestConstructBaseUnsupportedEngine verifies the negotiation error emitted when the engine does not support
// base construction.
func TestConstructBaseUnsupportedEngine(t *testing.T) {
	t.Parallel()

	// A plain testMonitor does not implement ConstructBaseResource, so GetDeploymentInfo omits CONSTRUCT_BASE.
	mocks := &testMonitor{}

	err := RunErr(func(ctx *Context) error {
		comp := &inheritanceDerived{}
		require.NoError(t, ctx.RegisterComponentResource("test:index:Derived", "d", comp))

		err := ctx.ConstructBase(comp, "test:index:Base", nil, "" /*packageRef*/)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a newer version of the Pulumi CLI")
		assert.Contains(t, err.Error(), "test:index:Derived")
		assert.Contains(t, err.Error(), "test:index:Base")

		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
}

// TestConstructBaseConcurrentIsolation runs two overlapping base constructs against a slow mock and verifies
// each resolves its own result. Reentrancy is safe by construction (goroutine + per-request Context); this
// guards against accidentally introducing shared mutable state.
func TestConstructBaseConcurrentIsolation(t *testing.T) {
	t.Parallel()

	// Block each ConstructBaseResource until both have arrived, forcing the two constructs to overlap.
	var arrived atomic.Int32
	release := make(chan struct{})
	mocks := &constructBaseMonitor{
		ConstructBaseResourceF: func(args MockConstructBaseArgs) (resource.PropertyMap, error) {
			if arrived.Add(1) == 2 {
				close(release)
			}
			<-release
			// Echo the URN back so cross-talk between contexts would be observable.
			return resource.PropertyMap{"out1": resource.NewProperty(args.URN)}, nil
		},
	}

	run := func(name string) error {
		return RunErr(func(ctx *Context) error {
			comp := &inheritanceDerived{}
			if err := ctx.RegisterComponentResource("test:index:Derived", name, comp); err != nil {
				return err
			}
			if err := ctx.ConstructBase(comp, "test:index:Base", nil, "" /*packageRef*/); err != nil {
				return err
			}
			v, _, _, _, err := await(comp.Out1)
			if err != nil {
				return err
			}
			urn, _, _, err := comp.URN().awaitURN(t.Context())
			if err != nil {
				return err
			}
			if v != string(urn) {
				return fmt.Errorf("cross-talk: base output %v does not match own URN %v", v, urn)
			}
			return nil
		}, WithMocks("project", "stack", mocks))
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, name := range []string{"a", "b"} {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			errs[i] = run(name)
		}(i, name)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
}
