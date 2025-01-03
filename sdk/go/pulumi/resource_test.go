// Copyright 2016-2021, Pulumi Corporation.
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
	"bytes"
	"context"
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	grpc "google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type testRes struct {
	CustomResourceState
	// equality identifier used for testing
	foo string
}

type testComp struct {
	ResourceState
}

type testProv struct {
	ProviderResourceState
	// equality identifier used for testing
	foo string
}

func TestResourceOptionMergingParent(t *testing.T) {
	t.Parallel()

	// last parent always wins, including nil values
	p1 := &testRes{foo: "a"}
	p2 := &testRes{foo: "b"}

	// two singleton options
	opts := merge(Parent(p1), Parent(p2))
	assert.Equal(t, p2, opts.Parent)

	// second parent nil
	opts = merge(Parent(p1), Parent(nil))
	assert.Equal(t, nil, opts.Parent)

	// first parent nil
	opts = merge(Parent(nil), Parent(p2))
	assert.Equal(t, p2, opts.Parent)
}

func TestResourceOptionMergingProvider(t *testing.T) {
	t.Parallel()

	// all providers are merged into a map
	// last specified provider for a given pkg wins
	aws1 := &testProv{foo: "a"}
	aws1.pkg = "aws"
	aws2 := &testProv{foo: "b"}
	aws2.pkg = "aws"
	azure := &testProv{foo: "c"}
	azure.pkg = "azure"

	t.Run("two singleton options for same pkg", func(t *testing.T) {
		t.Parallel()

		opts := merge(Provider(aws1), Provider(aws2))
		assert.Equal(t, 1, len(opts.Providers))
		assert.Equal(t, aws2, opts.Providers["aws"])
		assert.Equal(t, aws2, opts.Provider,
			"Provider should be set to the last specified provider")
	})

	t.Run("two singleton options for different pkg", func(t *testing.T) {
		t.Parallel()

		opts := merge(Provider(aws1), Provider(azure))
		assert.Equal(t, 2, len(opts.Providers))
		assert.Equal(t, aws1, opts.Providers["aws"])
		assert.Equal(t, azure, opts.Providers["azure"])
		assert.Equal(t, azure, opts.Provider,
			"Provider should be set to the last specified provider")
	})

	t.Run("singleton and array", func(t *testing.T) {
		t.Parallel()

		opts := merge(Provider(aws1), Providers(aws2, azure))
		assert.Equal(t, 2, len(opts.Providers))
		assert.Equal(t, aws2, opts.Providers["aws"])
		assert.Equal(t, azure, opts.Providers["azure"])
		assert.Equal(t, aws1, opts.Provider,
			"Provider should be set to the last specified provider")
	})

	t.Run("singleton and single value array", func(t *testing.T) {
		t.Parallel()

		opts := merge(Provider(aws1), Providers(aws2))
		assert.Equal(t, 1, len(opts.Providers))
		assert.Equal(t, aws2, opts.Providers["aws"])
		assert.Equal(t, aws1, opts.Provider,
			"Provider should be set to the last specified provider")
	})

	t.Run("two arrays", func(t *testing.T) {
		t.Parallel()

		opts := merge(Providers(aws1), Providers(azure))
		assert.Equal(t, 2, len(opts.Providers))
		assert.Equal(t, aws1, opts.Providers["aws"])
		assert.Equal(t, azure, opts.Providers["azure"])
		assert.Nil(t, opts.Provider,
			"Providers should not upgrade to Provider")
	})

	t.Run("overlapping arrays", func(t *testing.T) {
		t.Parallel()

		opts := merge(Providers(aws1, aws2), Providers(aws1, azure))
		assert.Equal(t, 2, len(opts.Providers))
		assert.Equal(t, aws1, opts.Providers["aws"])
		assert.Equal(t, azure, opts.Providers["azure"])
		assert.Nil(t, opts.Provider,
			"Providers should not upgrade to Provider")
	})

	m1 := map[string]ProviderResource{"aws": aws1}
	m2 := map[string]ProviderResource{"aws": aws2}
	m3 := map[string]ProviderResource{"aws": aws2, "azure": azure}

	t.Run("single value maps", func(t *testing.T) {
		t.Parallel()

		opts := merge(ProviderMap(m1), ProviderMap(m2))
		assert.Equal(t, 1, len(opts.Providers))
		assert.Equal(t, aws2, opts.Providers["aws"])
		assert.Nil(t, opts.Provider,
			"Providers should not upgrade to Provider")
	})

	t.Run("singleton with map", func(t *testing.T) {
		t.Parallel()

		opts := merge(Provider(aws1), ProviderMap(m3))
		assert.Equal(t, 2, len(opts.Providers))
		assert.Equal(t, aws2, opts.Providers["aws"])
		assert.Equal(t, azure, opts.Providers["azure"])
		assert.Equal(t, aws1, opts.Provider,
			"Provider should be set to the last specified provider")
	})

	t.Run("array and map", func(t *testing.T) {
		t.Parallel()

		opts := merge(Providers(aws2, aws1), ProviderMap(m3))
		assert.Equal(t, 2, len(opts.Providers))
		assert.Equal(t, aws2, opts.Providers["aws"])
		assert.Equal(t, azure, opts.Providers["azure"])
		assert.Nil(t, opts.Provider,
			"Providers should not upgrade to Provider")
	})
}

func TestResourceOptionMergingDependsOn(t *testing.T) {
	t.Parallel()

	// Depends on arrays are always appended together

	newRes := func(name string) (Resource, URN) {
		res := &testRes{foo: name}
		res.urn = CreateURN(String(name), String("t"), nil, String("stack"), String("project"))
		urn, _, _, err := res.urn.awaitURN(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return res, urn
	}

	d1, d1Urn := newRes("d1")
	d2, d2Urn := newRes("d2")
	d3, d3Urn := newRes("d3")

	resolveDependsOn := func(opts *resourceOptions) []URN {
		allDeps := map[URN]Resource{}
		for _, ds := range opts.DependsOn {
			if err := ds.addDeps(context.Background(), allDeps, nil /* from */); err != nil {
				t.Fatal(err)
			}
		}
		urns := maps.Keys(allDeps)
		sort.Slice(urns, func(i, j int) bool { return urns[i] < urns[j] })
		return urns
	}

	// two singleton options
	opts := merge(DependsOn([]Resource{d1}), DependsOn([]Resource{d2}))
	assert.Equal(t, []URN{d1Urn, d2Urn}, resolveDependsOn(opts))

	// nil d1
	opts = merge(DependsOn(nil), DependsOn([]Resource{d2}))
	assert.Equal(t, []URN{d2Urn}, resolveDependsOn(opts))

	// nil d2
	opts = merge(DependsOn([]Resource{d1}), DependsOn(nil))
	assert.Equal(t, []URN{d1Urn}, resolveDependsOn(opts))

	// multivalue arrays
	opts = merge(DependsOn([]Resource{d1, d2}), DependsOn([]Resource{d2, d3}))
	assert.Equal(t, []URN{d1Urn, d2Urn, d3Urn}, resolveDependsOn(opts))
}

func TestResourceOptionMergingProtect(t *testing.T) {
	t.Parallel()

	// last value wins
	opts := merge(Protect(true), Protect(false))
	assert.Equal(t, false, opts.Protect)
}

func TestResourceOptionMergingDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	// last value wins
	opts := merge(DeleteBeforeReplace(true), DeleteBeforeReplace(false))
	assert.Equal(t, false, opts.DeleteBeforeReplace)
}

func TestResourceOptionComposite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []ResourceOption
		want  *resourceOptions
	}{
		{
			name:  "no options",
			input: []ResourceOption{},
			want:  &resourceOptions{},
		},
		{
			name: "single option",
			input: []ResourceOption{
				DeleteBeforeReplace(true),
			},
			want: &resourceOptions{
				DeleteBeforeReplace: true,
			},
		},
		{
			name: "multiple conflicting options",
			input: []ResourceOption{
				DeleteBeforeReplace(true),
				DeleteBeforeReplace(false),
			},
			want: &resourceOptions{
				DeleteBeforeReplace: false,
			},
		},
		{
			name: "bouncing options",
			input: []ResourceOption{
				DeleteBeforeReplace(true),
				DeleteBeforeReplace(false),
				DeleteBeforeReplace(true),
			},
			want: &resourceOptions{
				DeleteBeforeReplace: true,
			},
		},
		{
			name: "different options",
			input: []ResourceOption{
				DeleteBeforeReplace(true),
				Protect(true),
			},
			want: &resourceOptions{
				DeleteBeforeReplace: true,
				Protect:             true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &resourceOptions{}
			Composite(tt.input...).applyResourceOption(opts)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestInvokeOptionComposite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []InvokeOption
		want  *invokeOptions
	}{
		{
			name:  "no options",
			input: []InvokeOption{},
			want:  &invokeOptions{},
		},
		{
			name: "single option",
			input: []InvokeOption{
				Version("test"),
			},
			want: &invokeOptions{
				Version: "test",
			},
		},
		{
			name: "multiple conflicting options",
			input: []InvokeOption{
				Version("test1"),
				Version("test2"),
			},
			want: &invokeOptions{
				Version: "test2",
			},
		},
		{
			name: "bouncing options",
			input: []InvokeOption{
				Version("test1"),
				Version("test2"),
				Version("test1"),
			},
			want: &invokeOptions{
				Version: "test1",
			},
		},
		{
			name: "different options",
			input: []InvokeOption{
				Version("test"),
				PluginDownloadURL("url"),
			},
			want: &invokeOptions{
				Version:           "test",
				PluginDownloadURL: "url",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &invokeOptions{}
			CompositeInvoke(tt.input...).applyInvokeOption(opts)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestResourceOptionMergingImport(t *testing.T) {
	t.Parallel()

	id1 := ID("a")
	id2 := ID("a")

	// last value wins
	opts := merge(Import(id1), Import(id2))
	assert.Equal(t, id2, opts.Import)

	// first import nil
	opts = merge(Import(nil), Import(id2))
	assert.Equal(t, id2, opts.Import)

	// second import nil
	opts = merge(Import(id1), Import(nil))
	assert.Equal(t, nil, opts.Import)
}

func TestResourceOptionMergingCustomTimeout(t *testing.T) {
	t.Parallel()

	c1 := &CustomTimeouts{Create: "1m"}
	c2 := &CustomTimeouts{Create: "2m"}
	var c3 *CustomTimeouts

	// last value wins
	opts := merge(Timeouts(c1), Timeouts(c2))
	assert.Equal(t, c2, opts.CustomTimeouts)

	// first import nil
	opts = merge(Timeouts(nil), Timeouts(c2))
	assert.Equal(t, c2, opts.CustomTimeouts)

	// second import nil
	opts = merge(Timeouts(c2), Timeouts(nil))
	assert.Equal(t, c3, opts.CustomTimeouts)
}

func TestResourceOptionMergingIgnoreChanges(t *testing.T) {
	t.Parallel()

	// IgnoreChanges arrays are always appended together
	i1 := "a"
	i2 := "b"
	i3 := "c"

	// two singleton options
	opts := merge(IgnoreChanges([]string{i1}), IgnoreChanges([]string{i2}))
	assert.Equal(t, []string{i1, i2}, opts.IgnoreChanges)

	// nil i1
	opts = merge(IgnoreChanges(nil), IgnoreChanges([]string{i2}))
	assert.Equal(t, []string{i2}, opts.IgnoreChanges)

	// nil i2
	opts = merge(IgnoreChanges([]string{i1}), IgnoreChanges(nil))
	assert.Equal(t, []string{i1}, opts.IgnoreChanges)

	// multivalue arrays
	opts = merge(IgnoreChanges([]string{i1, i2}), IgnoreChanges([]string{i2, i3}))
	assert.Equal(t, []string{i1, i2, i2, i3}, opts.IgnoreChanges)
}

func TestResourceOptionMergingAdditionalSecretOutputs(t *testing.T) {
	t.Parallel()

	// AdditionalSecretOutputs arrays are always appended together
	a1 := "a"
	a2 := "b"
	a3 := "c"

	// two singleton options
	opts := merge(AdditionalSecretOutputs([]string{a1}), AdditionalSecretOutputs([]string{a2}))
	assert.Equal(t, []string{a1, a2}, opts.AdditionalSecretOutputs)

	// nil a1
	opts = merge(AdditionalSecretOutputs(nil), AdditionalSecretOutputs([]string{a2}))
	assert.Equal(t, []string{a2}, opts.AdditionalSecretOutputs)

	// nil a2
	opts = merge(AdditionalSecretOutputs([]string{a1}), AdditionalSecretOutputs(nil))
	assert.Equal(t, []string{a1}, opts.AdditionalSecretOutputs)

	// multivalue arrays
	opts = merge(AdditionalSecretOutputs([]string{a1, a2}), AdditionalSecretOutputs([]string{a2, a3}))
	assert.Equal(t, []string{a1, a2, a2, a3}, opts.AdditionalSecretOutputs)
}

func TestResourceOptionMergingAliases(t *testing.T) {
	t.Parallel()

	// Aliases arrays are always appended together
	a1 := Alias{Name: String("a")}
	a2 := Alias{Name: String("b")}
	a3 := Alias{Name: String("c")}

	// two singleton options
	opts := merge(Aliases([]Alias{a1}), Aliases([]Alias{a2}))
	assert.Equal(t, []Alias{a1, a2}, opts.Aliases)

	// nil a1
	opts = merge(Aliases(nil), Aliases([]Alias{a2}))
	assert.Equal(t, []Alias{a2}, opts.Aliases)

	// nil a2
	opts = merge(Aliases([]Alias{a1}), Aliases(nil))
	assert.Equal(t, []Alias{a1}, opts.Aliases)

	// multivalue arrays
	opts = merge(Aliases([]Alias{a1, a2}), Aliases([]Alias{a2, a3}))
	assert.Equal(t, []Alias{a1, a2, a2, a3}, opts.Aliases)
}

func TestResourceOptionMergingTransformations(t *testing.T) {
	t.Parallel()

	// Transormations arrays are always appended together
	t1 := func(args *ResourceTransformationArgs) *ResourceTransformationResult {
		return &ResourceTransformationResult{}
	}
	t2 := func(args *ResourceTransformationArgs) *ResourceTransformationResult {
		return &ResourceTransformationResult{}
	}
	t3 := func(args *ResourceTransformationArgs) *ResourceTransformationResult {
		return &ResourceTransformationResult{}
	}

	// two singleton options
	opts := merge(Transformations([]ResourceTransformation{t1}), Transformations([]ResourceTransformation{t2}))
	assertTransformations(t, []ResourceTransformation{t1, t2}, opts.Transformations)

	// nil t1
	opts = merge(Transformations(nil), Transformations([]ResourceTransformation{t2}))
	assertTransformations(t, []ResourceTransformation{t2}, opts.Transformations)

	// nil t2
	opts = merge(Transformations([]ResourceTransformation{t1}), Transformations(nil))
	assertTransformations(t, []ResourceTransformation{t1}, opts.Transformations)

	// multivalue arrays
	opts = merge(Transformations([]ResourceTransformation{t1, t2}), Transformations([]ResourceTransformation{t2, t3}))
	assertTransformations(t, []ResourceTransformation{t1, t2, t2, t3}, opts.Transformations)
}

func assertTransformations(t *testing.T, t1 []ResourceTransformation, t2 []ResourceTransformation) {
	assert.Equal(t, len(t1), len(t2))
	for i := range t1 {
		p1 := reflect.ValueOf(t1[i]).Pointer()
		p2 := reflect.ValueOf(t2[i]).Pointer()
		assert.Equal(t, p1, p2)
	}
}

func TestResourceOptionMergingReplaceOnChanges(t *testing.T) {
	t.Parallel()

	// ReplaceOnChanges arrays are always appended together
	i1 := "a"
	i2 := "b"
	i3 := "c"

	// two singleton options
	opts := merge(ReplaceOnChanges([]string{i1}), ReplaceOnChanges([]string{i2}))
	assert.Equal(t, []string{i1, i2}, opts.ReplaceOnChanges)

	// nil i1
	opts = merge(ReplaceOnChanges(nil), ReplaceOnChanges([]string{i2}))
	assert.Equal(t, []string{i2}, opts.ReplaceOnChanges)

	// nil i2
	opts = merge(ReplaceOnChanges([]string{i1}), ReplaceOnChanges(nil))
	assert.Equal(t, []string{i1}, opts.ReplaceOnChanges)

	// multivalue arrays
	opts = merge(ReplaceOnChanges([]string{i1, i2}), ReplaceOnChanges([]string{i2, i3}))
	assert.Equal(t, []string{i1, i2, i2, i3}, opts.ReplaceOnChanges)
}

func TestNewResourceInput(t *testing.T) {
	t.Parallel()

	var resource Resource = &testRes{foo: "abracadabra"}
	resourceInput := NewResourceInput(resource)

	resourceOutput := resourceInput.ToResourceOutput()

	channel := make(chan interface{})
	resourceOutput.ApplyT(func(res interface{}) interface{} {
		channel <- res
		return res
	})

	res := <-channel
	unpackedRes, castOk := res.(*testRes)
	assert.Equal(t, true, castOk)
	assert.Equal(t, "abracadabra", unpackedRes.foo)
}

// Verifies that a Parent resource that has not been initialized will panic,
// and will instead report a meaningful error message.
func TestUninitializedParentResource(t *testing.T) {
	t.Parallel()

	type myComponent struct {
		ResourceState
	}

	type myCustomResource struct {
		CustomResourceState
	}

	tests := []struct {
		desc   string
		parent Resource

		// additional options to pass to RegisterResource
		// besides the Parent.
		// The original report of the panic was with an Alias option.
		opts []ResourceOption

		// Message that should be part of the error message.
		wantErr string
	}{
		{
			desc:   "component resource",
			parent: &myComponent{},
			wantErr: "WARNING: Ignoring component resource *pulumi.myComponent " +
				"(parent of my-resource :: test:index:MyResource) " +
				"because it was not registered with RegisterComponentResource",
		},
		{
			desc:   "component resource/alias",
			parent: &myComponent{},
			opts: []ResourceOption{
				Aliases([]Alias{
					{Name: String("alias1")},
				}),
			},
			wantErr: "WARNING: Ignoring component resource *pulumi.myComponent " +
				"(parent of my-resource :: test:index:MyResource) " +
				"because it was not registered with RegisterComponentResource",
		},
		{
			desc:   "custom resource",
			parent: &myCustomResource{},
			wantErr: "WARNING: Ignoring resource *pulumi.myCustomResource " +
				"(parent of my-resource :: test:index:MyResource) " +
				"because it was not registered with RegisterResource",
		},
		{
			desc:   "custom resource/alias",
			parent: &myCustomResource{},
			opts: []ResourceOption{
				Aliases([]Alias{
					{Name: String("alias1")},
				}),
			},
			wantErr: "WARNING: Ignoring resource *pulumi.myCustomResource " +
				"(parent of my-resource :: test:index:MyResource) " +
				"because it was not registered with RegisterResource",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			require.NotEmpty(t, tt.wantErr,
				"test case must specify an error message")

			err := RunErr(func(ctx *Context) error {
				// This is a hack.
				// We're accesing context mock internals
				// because the mock API does not expose a way
				// to set the logger.
				//
				// If this ever becomes a problem,
				// add a way to supply a logger to the mock
				// and use that here.
				var buff bytes.Buffer
				ctx.state.engine.(*mockEngine).logger = log.New(&buff, "", 0)

				opts := []ResourceOption{Parent(tt.parent)}
				opts = append(opts, tt.opts...)

				var res testRes
				require.NoError(t, ctx.RegisterResource(
					"test:index:MyResource",
					"my-resource",
					nil /* props */, &res, opts...))
				assert.Contains(t, buff.String(), tt.wantErr)
				return nil
			}, WithMocks("project", "stack", &testMonitor{}))
			assert.NoError(t, err)
		})
	}
}

func TestDependsOnInputs(t *testing.T) {
	t.Parallel()

	t.Run("known", func(t *testing.T) {
		t.Parallel()

		depTracker := &dependenciesTracker{}
		err := RunErr(func(ctx *Context) error {
			dep1 := newTestRes(t, ctx, "dep1")
			dep2 := newTestRes(t, ctx, "dep2")

			output := outputDependingOnResource(dep1, true).
				ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

			opts := DependsOnInputs(NewResourceArrayOutput(output))

			res := newTestRes(t, ctx, "res", opts)
			assertHasDeps(t, ctx, depTracker, res, dep1, dep2)
			return nil
		}, WithMocks("project", "stack", &testMonitor{}), WrapResourceMonitorClient(depTracker.Wrap))
		assert.NoError(t, err)
	})

	t.Run("dynamic", func(t *testing.T) {
		t.Parallel()

		depTracker := &dependenciesTracker{}
		err := RunErr(func(ctx *Context) error {
			checkDeps := func(name string, dependsOn ResourceArrayInput, expectedDeps ...Resource) {
				res := newTestRes(t, ctx, name, DependsOnInputs(dependsOn))
				assertHasDeps(t, ctx, depTracker, res, expectedDeps...)
			}

			dep1 := newTestRes(t, ctx, "dep1")
			dep2 := newTestRes(t, ctx, "dep2")
			dep3 := newTestRes(t, ctx, "dep3")

			out := outputDependingOnResource(dep1, true).
				ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

			checkDeps("r1", NewResourceArray(dep1, dep2), dep1, dep2)
			checkDeps("r2", NewResourceArrayOutput(out), dep1, dep2)
			checkDeps("r3", NewResourceArrayOutput(out, NewResourceOutput(dep3)), dep1, dep2, dep3)

			dep4 := newTestRes(t, ctx, "dep4")
			out4 := outputDependingOnResource(dep4, true).
				ApplyT(func(int) []Resource { return []Resource{dep1, dep2} }).(ResourceArrayInput)
			checkDeps("r4", out4, dep1, dep2, dep4)

			return nil
		}, WithMocks("project", "stack", &testMonitor{}), WrapResourceMonitorClient(depTracker.Wrap))
		assert.NoError(t, err)
	})
}

// https://github.com/pulumi/pulumi/issues/12161
func TestComponentResourcePropagatesProvider(t *testing.T) {
	t.Parallel()

	t.Run("provider option", func(t *testing.T) {
		t.Parallel()

		err := RunErr(func(ctx *Context) error {
			var prov struct{ ProviderResourceState }
			require.NoError(t,
				ctx.RegisterResource("pulumi:providers:test", "prov", nil /* props */, &prov),
				"error registering provider")

			var comp struct{ ResourceState }
			require.NoError(t,
				ctx.RegisterComponentResource("custom:foo:Component", "comp", &comp, Provider(&prov)),
				"error registering component")

			var custom struct{ ResourceState }
			require.NoError(t,
				ctx.RegisterResource("test:index:MyResource", "custom", nil /* props */, &custom, Parent(&comp)),
				"error registering resource")

			assert.True(t, &prov == custom.provider, "provider not propagated: %v", custom.provider)
			return nil
		}, WithMocks("project", "stack", &testMonitor{}))
		assert.NoError(t, err)
	})

	t.Run("providers option", func(t *testing.T) {
		t.Parallel()

		err := RunErr(func(ctx *Context) error {
			var prov struct{ ProviderResourceState }
			require.NoError(t,
				ctx.RegisterResource("pulumi:providers:test", "prov", nil /* props */, &prov),
				"error registering provider")

			var comp struct{ ResourceState }
			require.NoError(t,
				ctx.RegisterComponentResource("custom:foo:Component", "comp", &comp, Providers(&prov)),
				"error registering component")

			var custom struct{ ResourceState }
			require.NoError(t,
				ctx.RegisterResource("test:index:MyResource", "custom", nil /* props */, &custom, Parent(&comp)),
				"error registering resource")

			assert.True(t, &prov == custom.provider, "provider not propagated: %v", custom.provider)
			return nil
		}, WithMocks("project", "stack", &testMonitor{}))
		assert.NoError(t, err)
	})
}

// Verifies that if we pass an explicit provider to the provider plugin
// via the Provider() option,
// that the provider propagates this down to its children.
//
// Regression test for https://github.com/pulumi/pulumi/issues/12430
func TestRemoteComponentResourcePropagatesProvider(t *testing.T) {
	t.Parallel()

	err := RunErr(func(ctx *Context) error {
		var prov struct{ ProviderResourceState }
		require.NoError(t,
			ctx.RegisterResource("pulumi:providers:aws", "myprovider", nil /* props */, &prov),
			"error registering provider")

		var comp struct{ ResourceState }
		require.NoError(t,
			ctx.RegisterRemoteComponentResource("awsx:ec2:Vpc", "myvpc", nil /* props */, &comp, Provider(&prov)),
			"error registering component")

		var custom struct{ ResourceState }
		require.NoError(t,
			ctx.RegisterResource("aws:ec2/vpc:Vpc", "myvpc", nil /* props */, &custom, Parent(&comp)),
			"error registering resource")

		assert.True(t, &prov == custom.provider, "provider not propagated: %v", custom.provider)
		return nil
	}, WithMocks("project", "stack", &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			switch args.Name {
			case "myprovider":
				assert.Equal(t, "pulumi:providers:aws", args.RegisterRPC.Type)

			case "myvpc":
				// The remote component resource and the custom resource both
				// have the same name.
				//
				// However, only the custom resource should have the provider set.
				switch args.RegisterRPC.Type {
				case "awsx:ec2:Vpc":
					assert.Empty(t, args.Provider,
						"provider must not be set on remote component resource")

				case "aws:ec2/vpc:Vpc":
					assert.NotEmpty(t, args.Provider,
						"provider must be set on component resource")

				default:
					assert.Fail(t, "unexpected resource type: %s", args.RegisterRPC.Type)
				}
			}

			return args.Name, resource.PropertyMap{}, nil
		},
	}))
	assert.NoError(t, err)
}

// Verifies that Provider takes precedence over Providers.
func TestResourceProviderVersusProviders(t *testing.T) {
	t.Parallel()

	mocks := testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			return args.Name + "_id", args.Inputs, nil
		},
	}

	p1 := &testProv{
		ProviderResourceState: ProviderResourceState{pkg: "test"},
		foo:                   "1",
	}

	p2 := &testProv{
		ProviderResourceState: ProviderResourceState{pkg: "test"},
		foo:                   "2",
	}

	t.Run("singular, plural", func(t *testing.T) {
		t.Parallel()

		err := RunErr(func(ctx *Context) error {
			res := newTestRes(t, ctx, "myres", Provider(p1), Providers(p2))
			assert.Equal(t, p1, res.getProvider())

			return nil
		}, WithMocks("project", "stack", &mocks))
		require.NoError(t, err)
	})

	t.Run("plural, singular", func(t *testing.T) {
		t.Parallel()

		err := RunErr(func(ctx *Context) error {
			res := newTestRes(t, ctx, "myres", Providers(p2), Provider(p1))
			assert.Equal(t, p1, res.getProvider())

			return nil
		}, WithMocks("project", "stack", &mocks))
		require.NoError(t, err)
	})
}

// Verifies that multiple Provider options passed to a ComponentResource
// are inherited by its children.
//
// See also NOTE(Provider and Providers).
func TestComponentResourceMultipleSingletonProviders(t *testing.T) {
	t.Parallel()

	newTestProvider := func(ctx *Context, pkg, name string) ProviderResource {
		prov := testProv{foo: fmt.Sprintf("%s/%s", pkg, name)}
		prov.pkg = pkg

		require.NoError(t,
			ctx.RegisterResource("pulumi:providers:"+pkg, name, nil /* props */, &prov),
			"error registering provider")
		return &prov
	}

	newCustomResource := func(ctx *Context, typ, name string, opts ...ResourceOption) *testRes {
		res := testRes{foo: name}
		require.NoError(t,
			ctx.RegisterResource(typ, name, nil /* props */, &res, opts...))
		return &res
	}

	err := RunErr(func(ctx *Context) error {
		prov1 := newTestProvider(ctx, "pkg1", "prov1")
		prov2 := newTestProvider(ctx, "pkg2", "prov2")

		var component struct{ ResourceState }
		require.NoError(t, ctx.RegisterComponentResource(
			"my:foo:Component", "comp", &component, Provider(prov1), Provider(prov2)))

		res1 := newCustomResource(ctx, "pkg1:index:MyResource", "res1", Parent(&component))
		res2 := newCustomResource(ctx, "pkg2:index:MyResource", "res2", Parent(&component))

		assert.Equal(t, prov1, res1.provider, "provider 1 not propagated")
		assert.Equal(t, prov2, res2.provider, "provider 2 not propagated")

		return nil
	}, WithMocks("project", "stack", &testMonitor{}))
	assert.NoError(t, err)
}

func TestNewResourceOptions(t *testing.T) {
	t.Parallel()

	// Declared up here so that it may be shared to test
	// referential equality.
	sampleResourceInput := NewResourceInput(&testRes{foo: "foo"})

	tests := []struct {
		desc string
		give ResourceOption
		want ResourceOptions
	}{
		{
			desc: "AdditionalSecretOutputs",
			give: AdditionalSecretOutputs([]string{"foo"}),
			want: ResourceOptions{
				AdditionalSecretOutputs: []string{"foo"},
			},
		},
		{
			desc: "Aliases",
			give: Aliases([]Alias{
				{Name: String("foo")},
			}),
			want: ResourceOptions{
				Aliases: []Alias{
					{Name: String("foo")},
				},
			},
		},
		{
			desc: "Aliases/multiple options",
			give: Composite(
				Aliases([]Alias{{Name: String("foo")}}),
				Aliases([]Alias{{Name: String("bar")}}),
			),
			want: ResourceOptions{
				Aliases: []Alias{
					{Name: String("foo")},
					{Name: String("bar")},
				},
			},
		},
		{
			desc: "DeleteBeforeReplace",
			give: DeleteBeforeReplace(true),
			want: ResourceOptions{DeleteBeforeReplace: true},
		},
		{
			desc: "DependsOn",
			give: DependsOn([]Resource{
				&testRes{foo: "foo"},
				&testRes{foo: "bar"},
			}),
			want: ResourceOptions{
				DependsOn: []Resource{
					&testRes{foo: "foo"},
					&testRes{foo: "bar"},
				},
			},
		},
		{
			desc: "DependsOnInputs",
			give: DependsOnInputs(
				ResourceArray{sampleResourceInput},
			),
			want: ResourceOptions{
				DependsOnInputs: []ResourceArrayInput{
					ResourceArray{sampleResourceInput},
				},
			},
		},
		{
			desc: "IgnoreChanges",
			give: IgnoreChanges([]string{"foo"}),
			want: ResourceOptions{
				IgnoreChanges: []string{"foo"},
			},
		},
		{
			desc: "Import",
			give: Import(ID("bar")),
			want: ResourceOptions{Import: ID("bar")},
		},
		{
			desc: "Parent",
			give: Parent(&testRes{foo: "foo"}),
			want: ResourceOptions{
				Parent: &testRes{foo: "foo"},
			},
		},
		{
			desc: "Protect",
			give: Protect(true),
			want: ResourceOptions{Protect: true},
		},
		{
			desc: "Provider",
			give: Provider(&testProv{foo: "bar"}),
			want: ResourceOptions{
				Provider: &testProv{foo: "bar"},
				Providers: []ProviderResource{
					&testProv{foo: "bar"},
				},
			},
		},
		{
			desc: "ProviderMap",
			give: ProviderMap(map[string]ProviderResource{
				"foo": &testProv{
					ProviderResourceState: ProviderResourceState{pkg: "foo"},
					foo:                   "a",
				},
				"bar": &testProv{
					ProviderResourceState: ProviderResourceState{pkg: "bar"},
					foo:                   "b",
				},
			}),
			want: ResourceOptions{
				Providers: []ProviderResource{
					&testProv{
						ProviderResourceState: ProviderResourceState{pkg: "bar"},
						foo:                   "b",
					},
					&testProv{
						ProviderResourceState: ProviderResourceState{pkg: "foo"},
						foo:                   "a",
					},
				},
			},
		},
		{
			desc: "Providers",
			give: Providers(
				&testProv{
					ProviderResourceState: ProviderResourceState{pkg: "foo"},
					foo:                   "a",
				},
				&testProv{
					ProviderResourceState: ProviderResourceState{pkg: "bar"},
					foo:                   "b",
				},
			),
			want: ResourceOptions{
				Providers: []ProviderResource{
					&testProv{
						ProviderResourceState: ProviderResourceState{pkg: "bar"},
						foo:                   "b",
					},
					&testProv{
						ProviderResourceState: ProviderResourceState{pkg: "foo"},
						foo:                   "a",
					},
				},
			},
		},
		{
			desc: "ReplaceOnChanges",
			give: ReplaceOnChanges([]string{"foo", "bar"}),
			want: ResourceOptions{
				ReplaceOnChanges: []string{"foo", "bar"},
			},
		},
		{
			desc: "Timeouts",
			give: Timeouts(&CustomTimeouts{Create: "10s"}),
			want: ResourceOptions{
				CustomTimeouts: &CustomTimeouts{Create: "10s"},
			},
		},
		{
			desc: "URN",
			give: URN_("foo::bar"),
			want: ResourceOptions{URN: "foo::bar"},
		},
		{
			desc: "Version",
			give: Version("1.2.3"),
			want: ResourceOptions{Version: "1.2.3"},
		},
		{
			desc: "PluginDownloadURL",
			give: PluginDownloadURL("https://example.com/whatever"),
			want: ResourceOptions{PluginDownloadURL: "https://example.com/whatever"},
		},
		{
			desc: "RetainOnDelete",
			give: RetainOnDelete(true),
			want: ResourceOptions{RetainOnDelete: true},
		},
		{
			desc: "DeletedWith",
			give: DeletedWith(&testRes{foo: "a"}),
			want: ResourceOptions{DeletedWith: &testRes{foo: "a"}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got, err := NewResourceOptions(tt.give)
			require.NoError(t, err)
			assert.Equal(t, &tt.want, got)
		})
	}

	// Not covered in the table above because function pointers
	// cannot be compared.
	t.Run("Transformations", func(t *testing.T) {
		t.Parallel()

		var called bool
		tr := ResourceTransformation(func(args *ResourceTransformationArgs) *ResourceTransformationResult {
			called = true
			return &ResourceTransformationResult{}
		})

		ropts, err := NewResourceOptions(Transformations([]ResourceTransformation{tr}))
		require.NoError(t, err)
		require.Len(t, ropts.Transformations, 1)
		ropts.Transformations[0](&ResourceTransformationArgs{})
		assert.True(t, called, "Transformation function was not called")
	})
}

func TestNewInvokeOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give InvokeOption
		want InvokeOptions
	}{
		{
			desc: "Parent",
			give: Parent(&testRes{foo: "foo"}),
			want: InvokeOptions{
				Parent: &testRes{foo: "foo"},
			},
		},
		{
			desc: "Provider",
			give: Provider(&testProv{foo: "bar"}),
			want: InvokeOptions{
				Provider: &testProv{foo: "bar"},
			},
		},
		{
			desc: "Version",
			give: Version("1.2.3"),
			want: InvokeOptions{Version: "1.2.3"},
		},
		{
			desc: "PluginDownloadURL",
			give: PluginDownloadURL("https://example.com/whatever"),
			want: InvokeOptions{PluginDownloadURL: "https://example.com/whatever"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got, err := NewInvokeOptions(tt.give)
			require.NoError(t, err)
			assert.Equal(t, &tt.want, got)
		})
	}
}

func assertHasDeps(
	t *testing.T,
	ctx *Context,
	depTracker *dependenciesTracker,
	res Resource,
	expectedDeps ...Resource,
) {
	name := res.getName()
	resDeps := depTracker.dependencies(urnForRes(t, ctx, res))

	expDeps := slice.Prealloc[URN](len(expectedDeps))
	for _, expDepRes := range expectedDeps {
		expDep := urnForRes(t, ctx, expDepRes)
		expDeps = append(expDeps, expDep)
		assert.Containsf(t, resDeps, expDep, "Resource %s does not depend on %s",
			name, expDep)
	}

	for _, actualDep := range resDeps {
		assert.Containsf(t, expDeps, actualDep, "Resource %s unexpectedly depend on %s",
			name, actualDep)
	}
}

func outputDependingOnResource(res Resource, isKnown bool) IntOutput {
	out := newIntOutput()
	internal.ResolveOutput(out, 0, isKnown, false, resourcesToInternal([]Resource{res})) /* secret */
	return out
}

func newTestRes(t *testing.T, ctx *Context, name string, opts ...ResourceOption) Resource {
	var res testRes
	err := ctx.RegisterResource(fmt.Sprintf("test:resource:%stype", name), name, nil, &res, opts...)
	assert.NoError(t, err)
	return &res
}

func newUnknwonRes() *testRes {
	r := testRes{}
	r.id = IDOutput{} // Make the id unknown
	return &r
}

func urnForRes(t *testing.T, ctx *Context, res Resource) URN {
	urn, _, _, err := res.URN().awaitURN(ctx.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return urn
}

// dependenciesTracker tracks dependencies for registered resources.
//
// The zero value of dependenciesTracker is ready to use.
type dependenciesTracker struct {
	dependsOn sync.Map
}

// Wrap wraps a ResourceMonitorClient to start tracking RegisterResource calls
// sent through it.
//
// Use this with the WrapResourceMonitorClient option.
//
//	var dt dependenciesTracker
//	RunErr(..., WrapResourceMonitorClient(dt.Wrap))
func (dt *dependenciesTracker) Wrap(cl pulumirpc.ResourceMonitorClient) pulumirpc.ResourceMonitorClient {
	m := newInterceptingResourceMonitor(cl)
	m.afterRegisterResource = func(in *pulumirpc.RegisterResourceRequest,
		resp *pulumirpc.RegisterResourceResponse,
		err error,
	) {
		var deps []URN
		for _, dep := range in.GetDependencies() {
			deps = append(deps, URN(dep))
		}
		dt.dependsOn.Store(URN(resp.Urn), deps)
	}
	return m
}

func (dt *dependenciesTracker) dependencies(resource URN) []URN {
	val, ok := dt.dependsOn.Load(resource)
	if !ok {
		return nil
	}
	urns, ok := val.([]URN)
	if !ok {
		return nil
	}
	return urns
}

type interceptingResourceMonitor struct {
	pulumirpc.ResourceMonitorClient

	afterRegisterResource func(req *pulumirpc.RegisterResourceRequest, resp *pulumirpc.RegisterResourceResponse, err error)
}

func newInterceptingResourceMonitor(inner pulumirpc.ResourceMonitorClient) *interceptingResourceMonitor {
	return &interceptingResourceMonitor{
		ResourceMonitorClient: inner,
	}
}

func (i *interceptingResourceMonitor) RegisterResource(
	ctx context.Context,
	in *pulumirpc.RegisterResourceRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.RegisterResourceResponse, error) {
	resp, err := i.ResourceMonitorClient.RegisterResource(ctx, in, opts...)
	if i.afterRegisterResource != nil {
		i.afterRegisterResource(in, resp, err)
	}
	return resp, err
}

func TestRehydratedComponentConsideredRemote(t *testing.T) {
	t.Parallel()

	err := RunErr(func(ctx *Context) error {
		var component testComp
		require.NoError(t, ctx.RegisterComponentResource(
			"test:index:MyComponent",
			"component",
			&component))
		require.False(t, component.keepDependency())

		urn, _, _, err := component.URN().awaitURN(context.Background())
		require.NoError(t, err)

		var rehydrated testComp
		require.NoError(t, ctx.RegisterResource(
			"test:index:MyComponent",
			"component",
			nil,
			&rehydrated,
			URN_(string(urn))))
		require.True(t, rehydrated.keepDependency())

		return nil
	}, WithMocks("project", "stack", &testMonitor{}))
	require.NoError(t, err)
}

// Regression test for https://github.com/pulumi/pulumi/issues/12032
func TestParentAndDependsOnAreTheSame12032(t *testing.T) {
	t.Parallel()

	err := RunErr(func(ctx *Context) error {
		var parent testComp
		require.NoError(t, ctx.RegisterComponentResource(
			"pkg:index:first",
			"first",
			&parent))
		var child testComp
		require.NoError(t, ctx.RegisterComponentResource(
			"pkg:index:second",
			"second",
			&child,
			Parent(&parent),
			DependsOn([]Resource{&parent})))

		// This would freeze before the fix.
		var custom testRes
		require.NoError(t, ctx.RegisterResource(
			"foo:bar:baz",
			"myresource",
			nil,
			&custom,
			Parent(&child)))
		return nil
	}, WithMocks("project", "stack", &testMonitor{}))
	require.NoError(t, err)
}

type DoEchoResult struct {
	Echo *string `pulumi:"echo"`
}

type DoEchoArgs struct {
	Echo *string `pulumi:"echo"`
}

type DoEchoResultOutput struct{ *OutputState }

func (DoEchoResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*DoEchoResult)(nil)).Elem()
}

func TestInvokeDependsOn(t *testing.T) {
	t.Parallel()

	resolved := false

	monitor := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			return resource.PropertyMap{
				"echo": resource.NewStringProperty("hello"),
			}, nil
		},
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			time.Sleep(1 * time.Second)
			resolved = true
			return args.Name + "_id", nil, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		var args DoEchoArgs
		dep := newTestRes(t, ctx, "dep")
		opt := DependsOn([]Resource{dep})

		o := ctx.InvokeOutput("pkg:index:doEcho", args, DoEchoResultOutput{}, InvokeOutputOptions{
			InvokeOptions: []InvokeOption{opt},
		})

		v, known, secret, deps, err := internal.AwaitOutput(ctx.Context(), o)
		require.NoError(t, err)
		require.Equal(t, "hello", *v.(DoEchoResult).Echo)
		require.True(t, known)
		require.False(t, secret)
		require.True(t, resolved)
		require.Len(t, deps, 1)
		require.Equal(t, dep.URN(), deps[0].(Resource).URN())

		return nil
	}, WithMocks("project", "stack", monitor))
	require.NoError(t, err)
}

func TestInvokeDependsOnInputs(t *testing.T) {
	t.Parallel()

	resolved := false

	monitor := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			return resource.PropertyMap{
				"echo": resource.NewStringProperty("hello"),
			}, nil
		},
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			time.Sleep(1 * time.Second)
			resolved = true
			return args.Name + "_id", nil, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		var args DoEchoArgs
		dep := newTestRes(t, ctx, "dep")
		ro := NewResourceOutput(dep)
		opt := DependsOnInputs(NewResourceArrayOutput(ro))

		o := ctx.InvokeOutput("pkg:index:doEcho", args, DoEchoResultOutput{}, InvokeOutputOptions{
			InvokeOptions: []InvokeOption{opt},
		})

		v, known, secret, deps, err := internal.AwaitOutput(ctx.Context(), o)
		require.NoError(t, err)
		require.Equal(t, "hello", *v.(DoEchoResult).Echo)
		require.True(t, known)
		require.False(t, secret)
		require.True(t, resolved)
		require.Len(t, deps, 1)
		require.Equal(t, dep.URN(), deps[0].(Resource).URN())

		return nil
	}, WithMocks("project", "stack", monitor))
	require.NoError(t, err)
}

func TestInvokeDependsOnUnknown(t *testing.T) {
	t.Parallel()

	monitor := &testMonitor{}

	err := RunErr(func(ctx *Context) error {
		var args DoEchoArgs
		unknownDep := newUnknwonRes()

		o := ctx.InvokeOutput("pkg:index:doEcho", args, DoEchoResultOutput{}, InvokeOutputOptions{
			InvokeOptions: []InvokeOption{DependsOn([]Resource{unknownDep})},
		})

		_, known, secret, deps, err := internal.AwaitOutput(ctx.Context(), o)
		require.NoError(t, err)
		require.False(t, known)
		require.False(t, secret)
		require.Len(t, deps, 1)
		require.True(t, deps[0] == unknownDep)

		return nil
	}, WithMocks("project", "stack", monitor))
	require.NoError(t, err)
}

func TestInvokeDependsOnUnknownChild(t *testing.T) {
	t.Parallel()

	monitor := &testMonitor{}

	err := RunErr(func(ctx *Context) error {
		var args DoEchoArgs
		unknownDep := newUnknwonRes()
		comp := &testComp{}
		comp.children = resourceSet{}
		comp.children.add(unknownDep)

		o := ctx.InvokeOutput("pkg:index:doEcho", args, DoEchoResultOutput{}, InvokeOutputOptions{
			InvokeOptions: []InvokeOption{DependsOn([]Resource{comp})},
		})

		_, known, secret, deps, err := internal.AwaitOutput(ctx.Context(), o)
		require.NoError(t, err)
		require.False(t, known)
		require.False(t, secret)
		require.Len(t, deps, 1)
		require.True(t, deps[0] == comp) // The component, not the child

		return nil
	}, WithMocks("project", "stack", monitor))
	require.NoError(t, err)
}

func TestInvokeDependsOnIgnored(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})

	monitor := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			return resource.PropertyMap{
				"echo": resource.NewStringProperty("hello"),
			}, nil
		},
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			// Wait to resolve the resource until after the invoke has completed.
			// This lets us test that the invoke did not wait for this resource.
			<-done
			return args.Name + "_id", nil, nil
		},
	}

	// If we do not ignore DependsOn for direct form invokes, this test will hang.
	// We'll run the test in a goroutine and timeout if it takes too long.
	testDone := make(chan struct{})
	go func() {
		err := RunErr(func(ctx *Context) error {
			var rv DoEchoResult
			var args DoEchoArgs
			dep := newTestRes(t, ctx, "dep")
			ro := NewResourceOutput(dep)
			opts := DependsOnInputs(NewResourceArrayOutput(ro))

			err := ctx.InvokePackage("pkg:index:doEcho", args, &rv, "some-package-ref", opts)
			require.NoError(t, err)

			done <- struct{}{}

			return nil
		}, WithMocks("project", "stack", monitor))
		require.NoError(t, err)

		testDone <- struct{}{}
	}()

	select {
	case <-testDone:
	case <-time.After(2 * time.Second):
		t.Fatal("test timed out")
	}
}

func TestInvokeSecret(t *testing.T) {
	t.Parallel()

	monitor := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			return resource.PropertyMap{
				// The invoke result contains a secret.
				"echo": resource.MakeSecret(resource.NewStringProperty("hello")),
			}, nil
		},
	}

	// InvokePackageRaw
	err := RunErr(func(ctx *Context) error {
		var rv DoEchoResult
		var args DoEchoArgs
		dep := newTestRes(t, ctx, "dep")
		ro := NewResourceOutput(dep)
		opts := DependsOnInputs(NewResourceArrayOutput(ro))

		isSecret, err := ctx.InvokePackageRaw("pkg:index:doEcho", args, &rv, "some-package-ref", opts)
		require.NoError(t, err)
		require.True(t, isSecret)
		require.Equal(t, "hello", *rv.Echo)

		return nil
	}, WithMocks("project", "stack", monitor))
	require.NoError(t, err)

	// InvokeOutput
	err = RunErr(func(ctx *Context) error {
		var args DoEchoArgs
		dep := newTestRes(t, ctx, "dep")
		ro := NewResourceOutput(dep)
		opt := DependsOnInputs(NewResourceArrayOutput(ro))

		o := ctx.InvokeOutput("pkg:index:doEcho", args, DoEchoResultOutput{}, InvokeOutputOptions{
			InvokeOptions: []InvokeOption{opt},
		})

		v, known, secret, deps, err := internal.AwaitOutput(ctx.Context(), o)
		require.NoError(t, err)
		require.Equal(t, "hello", *v.(DoEchoResult).Echo)
		require.True(t, known)
		require.True(t, secret)
		require.Len(t, deps, 1)
		require.Equal(t, dep.URN(), deps[0].(Resource).URN())

		return nil
	}, WithMocks("project", "stack", monitor))
	require.NoError(t, err)
}
