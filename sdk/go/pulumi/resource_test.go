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
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/assert"
	grpc "google.golang.org/grpc"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type testRes struct {
	CustomResourceState
	// equality identifier used for testing
	foo string
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
	p1 := &testProv{foo: "a"}
	p1.pkg = "aws"
	p2 := &testProv{foo: "b"}
	p2.pkg = "aws"
	p3 := &testProv{foo: "c"}
	p3.pkg = "azure"

	// merges two singleton options for same pkg
	opts := merge(Provider(p1), Provider(p2))
	assert.Equal(t, 1, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])

	// merges two singleton options for different pkg
	opts = merge(Provider(p1), Provider(p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p1, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merges singleton and array
	opts = merge(Provider(p1), Providers(p2, p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merges singleton and single value array
	opts = merge(Provider(p1), Providers(p2))
	assert.Equal(t, 1, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])

	// merges two arrays
	opts = merge(Providers(p1), Providers(p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p1, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merges overlapping arrays
	opts = merge(Providers(p1, p2), Providers(p1, p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p1, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merge single value maps
	m1 := map[string]ProviderResource{"aws": p1}
	m2 := map[string]ProviderResource{"aws": p2}
	m3 := map[string]ProviderResource{"aws": p2, "azure": p3}

	// merge single value maps
	opts = merge(ProviderMap(m1), ProviderMap(m2))
	assert.Equal(t, 1, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])

	// merge singleton with map
	opts = merge(Provider(p1), ProviderMap(m3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merge arry and map
	opts = merge(Providers(p2, p1), ProviderMap(m3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])
}

func TestResourceOptionMergingDependsOn(t *testing.T) {
	t.Parallel()

	// Depends on arrays are always appended together

	newRes := func(name string) (Resource, URN) {
		res := &testRes{foo: name}
		res.urn = CreateURN(String(name), String("t"), nil, String("stack"), String("project"))
		urn, _, _, err := res.urn.awaitURN(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		return res, urn
	}

	d1, d1Urn := newRes("d1")
	d2, d2Urn := newRes("d2")
	d3, d3Urn := newRes("d3")

	resolveDependsOn := func(opts *resourceOptions) []URN {
		allDeps := urnSet{}
		for _, f := range opts.DependsOn {
			deps, err := f(context.TODO())
			if err != nil {
				t.Fatal(err)
			}
			allDeps.union(deps)
		}
		return allDeps.sortedValues()
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
	var resourceInput ResourceInput = NewResourceInput(resource)

	var resourceOutput ResourceOutput = resourceInput.ToResourceOutput()

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

func TestDependsOnInputs(t *testing.T) {
	t.Parallel()

	t.Run("known", func(t *testing.T) {
		t.Parallel()

		err := RunErr(func(ctx *Context) error {
			depTracker := trackDependencies(ctx)

			dep1 := newTestRes(t, ctx, "dep1")
			dep2 := newTestRes(t, ctx, "dep2")

			output := outputDependingOnResource(dep1, true).
				ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

			opts := DependsOnInputs(NewResourceArrayOutput(output))

			res := newTestRes(t, ctx, "res", opts)
			assertHasDeps(t, ctx, depTracker, res, dep1, dep2)
			return nil
		}, WithMocks("project", "stack", &testMonitor{}))
		assert.NoError(t, err)
	})

	t.Run("dynamic", func(t *testing.T) {
		t.Parallel()

		err := RunErr(func(ctx *Context) error {
			depTracker := trackDependencies(ctx)

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
		}, WithMocks("project", "stack", &testMonitor{}))
		assert.NoError(t, err)
	})
}

func assertHasDeps(
	t *testing.T,
	ctx *Context,
	depTracker *dependenciesTracker,
	res Resource,
	expectedDeps ...Resource) {

	name := res.getName()
	resDeps := depTracker.dependencies(urn(t, ctx, res))

	var expDeps []URN
	for _, expDepRes := range expectedDeps {
		expDep := urn(t, ctx, expDepRes)
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
	out.resolve(0, isKnown, false /* secret */, []Resource{res})
	return out
}

func newTestRes(t *testing.T, ctx *Context, name string, opts ...ResourceOption) Resource {
	var res testRes
	err := ctx.RegisterResource(fmt.Sprintf("test:resource:%stype", name), name, nil, &res, opts...)
	assert.NoError(t, err)
	return &res
}

func urn(t *testing.T, ctx *Context, res Resource) URN {
	urn, _, _, err := res.URN().awaitURN(ctx.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return urn
}

type dependenciesTracker struct {
	dependsOn *sync.Map
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

func trackDependencies(ctx *Context) *dependenciesTracker {
	dependsOn := &sync.Map{}
	m := newInterceptingResourceMonitor(ctx.monitor)
	m.afterRegisterResource = func(in *pulumirpc.RegisterResourceRequest,
		resp *pulumirpc.RegisterResourceResponse,
		err error) {
		var deps []URN
		for _, dep := range in.GetDependencies() {
			deps = append(deps, URN(dep))
		}
		dependsOn.Store(URN(resp.Urn), deps)
	}
	ctx.monitor = m
	return &dependenciesTracker{dependsOn}
}

type interceptingResourceMonitor struct {
	inner                 pulumirpc.ResourceMonitorClient
	afterRegisterResource func(req *pulumirpc.RegisterResourceRequest, resp *pulumirpc.RegisterResourceResponse, err error)
}

func newInterceptingResourceMonitor(inner pulumirpc.ResourceMonitorClient) *interceptingResourceMonitor {
	m := &interceptingResourceMonitor{}
	m.inner = inner
	return m
}

func (i *interceptingResourceMonitor) Call(
	ctx context.Context, req *pulumirpc.CallRequest, options ...grpc.CallOption) (*pulumirpc.CallResponse, error) {
	return i.inner.Call(ctx, req, options...)
}

func (i *interceptingResourceMonitor) SupportsFeature(ctx context.Context,
	in *pulumirpc.SupportsFeatureRequest,
	opts ...grpc.CallOption) (*pulumirpc.SupportsFeatureResponse, error) {
	return i.inner.SupportsFeature(ctx, in, opts...)
}

func (i *interceptingResourceMonitor) Invoke(ctx context.Context,
	in *pulumirpc.InvokeRequest,
	opts ...grpc.CallOption) (*pulumirpc.InvokeResponse, error) {
	return i.inner.Invoke(ctx, in, opts...)
}

func (i *interceptingResourceMonitor) StreamInvoke(ctx context.Context,
	in *pulumirpc.InvokeRequest,
	opts ...grpc.CallOption) (pulumirpc.ResourceMonitor_StreamInvokeClient, error) {
	return i.inner.StreamInvoke(ctx, in, opts...)
}

func (i *interceptingResourceMonitor) ReadResource(ctx context.Context,
	in *pulumirpc.ReadResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.ReadResourceResponse, error) {
	return i.inner.ReadResource(ctx, in, opts...)
}

func (i *interceptingResourceMonitor) RegisterResource(ctx context.Context,
	in *pulumirpc.RegisterResourceRequest,
	opts ...grpc.CallOption) (*pulumirpc.RegisterResourceResponse, error) {
	resp, err := i.inner.RegisterResource(ctx, in, opts...)
	if i.afterRegisterResource != nil {
		i.afterRegisterResource(in, resp, err)
	}
	return resp, err
}

func (i *interceptingResourceMonitor) RegisterResourceOutputs(ctx context.Context,
	in *pulumirpc.RegisterResourceOutputsRequest,
	opts ...grpc.CallOption) (*empty.Empty, error) {
	return i.inner.RegisterResourceOutputs(ctx, in, opts...)
}

var _ pulumirpc.ResourceMonitorClient = &interceptingResourceMonitor{}
