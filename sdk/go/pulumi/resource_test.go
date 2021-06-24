package pulumi

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
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
	// last parent always wins, including nil values
	p1 := &testRes{foo: "a"}
	p2 := &testRes{foo: "b"}

	// two singleton options
	opts := tmerge(t, Parent(p1), Parent(p2))
	assert.Equal(t, p2, opts.Parent)

	// second parent nil
	opts = tmerge(t, Parent(p1), Parent(nil))
	assert.Equal(t, nil, opts.Parent)

	// first parent nil
	opts = tmerge(t, Parent(nil), Parent(p2))
	assert.Equal(t, p2, opts.Parent)
}

func TestResourceOptionMergingProvider(t *testing.T) {
	// all providers are merged into a map
	// last specified provider for a given pkg wins
	p1 := &testProv{foo: "a"}
	p1.pkg = "aws"
	p2 := &testProv{foo: "b"}
	p2.pkg = "aws"
	p3 := &testProv{foo: "c"}
	p3.pkg = "azure"

	// merges two singleton options for same pkg
	opts := tmerge(t, Provider(p1), Provider(p2))
	assert.Equal(t, 1, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])

	// merges two singleton options for different pkg
	opts = tmerge(t, Provider(p1), Provider(p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p1, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merges singleton and array
	opts = tmerge(t, Provider(p1), Providers(p2, p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merges singleton and single value array
	opts = tmerge(t, Provider(p1), Providers(p2))
	assert.Equal(t, 1, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])

	// merges two arrays
	opts = tmerge(t, Providers(p1), Providers(p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p1, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merges overlapping arrays
	opts = tmerge(t, Providers(p1, p2), Providers(p1, p3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p1, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merge single value maps
	m1 := map[string]ProviderResource{"aws": p1}
	m2 := map[string]ProviderResource{"aws": p2}
	m3 := map[string]ProviderResource{"aws": p2, "azure": p3}

	// merge single value maps
	opts = tmerge(t, ProviderMap(m1), ProviderMap(m2))
	assert.Equal(t, 1, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])

	// merge singleton with map
	opts = tmerge(t, Provider(p1), ProviderMap(m3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])

	// merge arry and map
	opts = tmerge(t, Providers(p2, p1), ProviderMap(m3))
	assert.Equal(t, 2, len(opts.Providers))
	assert.Equal(t, p2, opts.Providers["aws"])
	assert.Equal(t, p3, opts.Providers["azure"])
}

func TestResourceOptionMergingDependsOn(t *testing.T) {
	// Depends on arrays are always appended together
	d1 := &testRes{foo: "a"}
	d2 := &testRes{foo: "b"}
	d3 := &testRes{foo: "c"}

	// two singleton options
	opts := tmerge(t, DependsOn([]Resource{d1}), DependsOn([]Resource{d2}))
	assert.Equal(t, []Resource{d1, d2}, opts.DependsOn)

	// nil d1
	opts = tmerge(t, DependsOn(nil), DependsOn([]Resource{d2}))
	assert.Equal(t, []Resource{d2}, opts.DependsOn)

	// nil d2
	opts = tmerge(t, DependsOn([]Resource{d1}), DependsOn(nil))
	assert.Equal(t, []Resource{d1}, opts.DependsOn)

	// multivalue arrays
	opts = tmerge(t, DependsOn([]Resource{d1, d2}), DependsOn([]Resource{d2, d3}))
	assert.Equal(t, []Resource{d1, d2, d2, d3}, opts.DependsOn)
}

func TestResourceOptionMergingProtect(t *testing.T) {
	// last value wins
	opts := tmerge(t, Protect(true), Protect(false))
	assert.Equal(t, false, opts.Protect)
}

func TestResourceOptionMergingDeleteBeforeReplace(t *testing.T) {
	// last value wins
	opts := tmerge(t, DeleteBeforeReplace(true), DeleteBeforeReplace(false))
	assert.Equal(t, false, opts.DeleteBeforeReplace)
}

func TestResourceOptionMergingImport(t *testing.T) {
	id1 := ID("a")
	id2 := ID("a")

	// last value wins
	opts := tmerge(t, Import(id1), Import(id2))
	assert.Equal(t, id2, opts.Import)

	// first import nil
	opts = tmerge(t, Import(nil), Import(id2))
	assert.Equal(t, id2, opts.Import)

	// second import nil
	opts = tmerge(t, Import(id1), Import(nil))
	assert.Equal(t, nil, opts.Import)
}

func TestResourceOptionMergingCustomTimeout(t *testing.T) {
	c1 := &CustomTimeouts{Create: "1m"}
	c2 := &CustomTimeouts{Create: "2m"}
	var c3 *CustomTimeouts

	// last value wins
	opts := tmerge(t, Timeouts(c1), Timeouts(c2))
	assert.Equal(t, c2, opts.CustomTimeouts)

	// first import nil
	opts = tmerge(t, Timeouts(nil), Timeouts(c2))
	assert.Equal(t, c2, opts.CustomTimeouts)

	// second import nil
	opts = tmerge(t, Timeouts(c2), Timeouts(nil))
	assert.Equal(t, c3, opts.CustomTimeouts)
}

func TestResourceOptionMergingIgnoreChanges(t *testing.T) {
	// IgnoreChanges arrays are always appended together
	i1 := "a"
	i2 := "b"
	i3 := "c"

	// two singleton options
	opts := tmerge(t, IgnoreChanges([]string{i1}), IgnoreChanges([]string{i2}))
	assert.Equal(t, []string{i1, i2}, opts.IgnoreChanges)

	// nil i1
	opts = tmerge(t, IgnoreChanges(nil), IgnoreChanges([]string{i2}))
	assert.Equal(t, []string{i2}, opts.IgnoreChanges)

	// nil i2
	opts = tmerge(t, IgnoreChanges([]string{i1}), IgnoreChanges(nil))
	assert.Equal(t, []string{i1}, opts.IgnoreChanges)

	// multivalue arrays
	opts = tmerge(t, IgnoreChanges([]string{i1, i2}), IgnoreChanges([]string{i2, i3}))
	assert.Equal(t, []string{i1, i2, i2, i3}, opts.IgnoreChanges)
}

func TestResourceOptionMergingAdditionalSecretOutputs(t *testing.T) {
	// AdditionalSecretOutputs arrays are always appended together
	a1 := "a"
	a2 := "b"
	a3 := "c"

	// two singleton options
	opts := tmerge(t, AdditionalSecretOutputs([]string{a1}), AdditionalSecretOutputs([]string{a2}))
	assert.Equal(t, []string{a1, a2}, opts.AdditionalSecretOutputs)

	// nil a1
	opts = tmerge(t, AdditionalSecretOutputs(nil), AdditionalSecretOutputs([]string{a2}))
	assert.Equal(t, []string{a2}, opts.AdditionalSecretOutputs)

	// nil a2
	opts = tmerge(t, AdditionalSecretOutputs([]string{a1}), AdditionalSecretOutputs(nil))
	assert.Equal(t, []string{a1}, opts.AdditionalSecretOutputs)

	// multivalue arrays
	opts = tmerge(t, AdditionalSecretOutputs([]string{a1, a2}), AdditionalSecretOutputs([]string{a2, a3}))
	assert.Equal(t, []string{a1, a2, a2, a3}, opts.AdditionalSecretOutputs)
}

func TestResourceOptionMergingAliases(t *testing.T) {
	// Aliases arrays are always appended together
	a1 := Alias{Name: String("a")}
	a2 := Alias{Name: String("b")}
	a3 := Alias{Name: String("c")}

	// two singleton options
	opts := tmerge(t, Aliases([]Alias{a1}), Aliases([]Alias{a2}))
	assert.Equal(t, []Alias{a1, a2}, opts.Aliases)

	// nil a1
	opts = tmerge(t, Aliases(nil), Aliases([]Alias{a2}))
	assert.Equal(t, []Alias{a2}, opts.Aliases)

	// nil a2
	opts = tmerge(t, Aliases([]Alias{a1}), Aliases(nil))
	assert.Equal(t, []Alias{a1}, opts.Aliases)

	// multivalue arrays
	opts = tmerge(t, Aliases([]Alias{a1, a2}), Aliases([]Alias{a2, a3}))
	assert.Equal(t, []Alias{a1, a2, a2, a3}, opts.Aliases)
}

func TestResourceOptionMergingTransformations(t *testing.T) {
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
	opts := tmerge(t, Transformations([]ResourceTransformation{t1}), Transformations([]ResourceTransformation{t2}))
	assertTransformations(t, []ResourceTransformation{t1, t2}, opts.Transformations)

	// nil t1
	opts = tmerge(t, Transformations(nil), Transformations([]ResourceTransformation{t2}))
	assertTransformations(t, []ResourceTransformation{t2}, opts.Transformations)

	// nil t2
	opts = tmerge(t, Transformations([]ResourceTransformation{t1}), Transformations(nil))
	assertTransformations(t, []ResourceTransformation{t1}, opts.Transformations)

	// multivalue arrays
	opts = tmerge(t, Transformations([]ResourceTransformation{t1, t2}), Transformations([]ResourceTransformation{t2, t3}))
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

func TestNewResourceInput(t *testing.T) {
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

func TestParentInput(t *testing.T) {
	err := RunErr(func(ctx *Context) error {
		dep := newTestRes(t, ctx, "resDependency")
		parent := newTestRes(t, ctx, "resParent")

		parentWithDep := ResourceOutput{
			Any(dep).
				ApplyT(func(interface{}) interface{} { return parent }).
				getState(),
		}

		child := newTestRes(t, ctx, "resChild", ParentInput(parentWithDep))

		m := ctx.monitor.(*mockMonitor)

		assert.Equalf(t, urn(t, ctx, parent), m.parents[urn(t, ctx, child)],
			"Failed to set parent via ParentInput")

		assert.Containsf(t, m.dependencies[urn(t, ctx, child)], urn(t, ctx, dep),
			"Failed to propagate dependencies via ParentInput")

		return nil
	}, WithMocks("project", "stack", &testMonitor{}))
	assert.NoError(t, err)
}

func TestDependsOnInputs(t *testing.T) {
	err := RunErr(func(ctx *Context) error {
		dep1 := newTestRes(t, ctx, "resDependency1")
		dep2 := newTestRes(t, ctx, "resDependency2")

		// Construct an output that resolve to `dep2` but also
		// implicitly depends on `dep1`.
		output := ResourceOutput{
			Any(dep1).
				ApplyT(func(interface{}) interface{} { return dep2 }).
				getState(),
		}

		res := newTestRes(t, ctx, "resDependent", DependsOnInputs([]ResourceInput{output}))

		m := ctx.monitor.(*mockMonitor)

		assert.Containsf(t, m.dependencies[urn(t, ctx, res)], urn(t, ctx, dep2),
			"Failed to propagate direct dependencies via DependsOnInputs")

		assert.Containsf(t, m.dependencies[urn(t, ctx, res)], urn(t, ctx, dep1),
			"Failed to propagate indirect dependencies via DependsOnInputs")

		return nil
	}, WithMocks("project", "stack", &testMonitor{}))
	assert.NoError(t, err)
}

func TestProviderInput(t *testing.T) {
	providerId := "providerId"
	providerUrnBase := resource.NewURN("stack", "project", "", "pulumi:providers:test", "test")
	providerUrn := fmt.Sprintf("%s::%s", providerUrnBase, providerId)

	var seenProviders []string

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			if args.Provider != "" {
				seenProviders = append(seenProviders, args.Provider)
			}
			return "freshID", nil, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		dep := newTestRes(t, ctx, "resDependency")

		var providerResource ProviderResource = newSimpleProviderResource(ctx, URN(providerUrnBase), "providerId")

		// Construct an output that resolve to `providerResource` but also depends on `dep`.
		output := Any(dep).
			ApplyT(func(interface{}) ProviderResource { return providerResource }).(ProviderResourceOutput)

		res := newTestRes(t, ctx, "resWithProvider", ProviderInput(output))

		m := ctx.monitor.(*mockMonitor)

		assert.Containsf(t, m.dependencies[urn(t, ctx, res)], urn(t, ctx, dep),
			"Failed to propagate indirect dependencies via ProviderInput")

		return nil
	}, WithMocks("project", "stack", mocks))

	assert.NoError(t, err)

	assert.Len(t, seenProviders, 1)
	assert.Equal(t, providerUrn, seenProviders[0])
}

func TestProviderInputs(t *testing.T) {
	providerId := "providerId"
	providerUrnBase := resource.NewURN("stack", "project", "", "pulumi:providers:test", "test")
	providerUrn := fmt.Sprintf("%s::%s", providerUrnBase, providerId)

	var seenProviders []string

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			if args.Provider != "" {
				seenProviders = append(seenProviders, args.Provider)
			}
			return "freshID", nil, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		dep := newTestRes(t, ctx, "resDependency")

		var providerResource ProviderResource = newSimpleProviderResource(ctx, URN(providerUrnBase), "providerId")

		// Construct an output that resolve to `providerResource` but also depends on `dep`.
		output := Any(dep).
			ApplyT(func(interface{}) ProviderResource { return providerResource }).(ProviderResourceOutput)

		res := newTestRes(t, ctx, "resWithProvider", ProviderInputs(output))

		m := ctx.monitor.(*mockMonitor)

		assert.Containsf(t, m.dependencies[urn(t, ctx, res)], urn(t, ctx, dep),
			"Failed to propagate indirect dependencies via ProviderInput")

		return nil
	}, WithMocks("project", "stack", mocks))

	assert.NoError(t, err)

	assert.Len(t, seenProviders, 1)
	assert.Equal(t, providerUrn, seenProviders[0])
}

func TestProviderInputMap(t *testing.T) {
	providerId := "providerId"
	providerUrnBase := resource.NewURN("stack", "project", "", "pulumi:providers:test", "test")
	providerUrn := fmt.Sprintf("%s::%s", providerUrnBase, providerId)

	var seenProviders []string

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			if args.Provider != "" {
				seenProviders = append(seenProviders, args.Provider)
			}
			return "freshID", nil, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		dep := newTestRes(t, ctx, "resDependency")

		var providerResource ProviderResource = newSimpleProviderResource(ctx, URN(providerUrnBase), "providerId")

		// Construct an output that resolve to `providerResource` but also depends on `dep`.
		output := Any(dep).
			ApplyT(func(interface{}) ProviderResource { return providerResource }).(ProviderResourceOutput)

		res := newTestRes(t, ctx, "resWithProvider", ProviderInputMap(map[string]ProviderResourceInput{
			providerResource.getPackage(): output,
		}))

		m := ctx.monitor.(*mockMonitor)

		assert.Containsf(t, m.dependencies[urn(t, ctx, res)], urn(t, ctx, dep),
			"Failed to propagate indirect dependencies via ProviderInput")

		return nil
	}, WithMocks("project", "stack", mocks))

	assert.NoError(t, err)

	assert.Len(t, seenProviders, 1)
	assert.Equal(t, providerUrn, seenProviders[0])
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

func tmerge(t *testing.T, opts ...ResourceOption) *resourceOptions {
	ro, err := merge(context.TODO(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return ro
}
