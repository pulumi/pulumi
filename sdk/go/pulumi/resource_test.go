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
	"reflect"
	"testing"

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
	// Depends on arrays are always appended together
	d1 := &testRes{foo: "a"}
	d2 := &testRes{foo: "b"}
	d3 := &testRes{foo: "c"}

	// two singleton options
	opts := merge(DependsOn([]Resource{d1}), DependsOn([]Resource{d2}))
	assert.Equal(t, []Resource{d1, d2}, opts.DependsOn)

	// nil d1
	opts = merge(DependsOn(nil), DependsOn([]Resource{d2}))
	assert.Equal(t, []Resource{d2}, opts.DependsOn)

	// nil d2
	opts = merge(DependsOn([]Resource{d1}), DependsOn(nil))
	assert.Equal(t, []Resource{d1}, opts.DependsOn)

	// multivalue arrays
	opts = merge(DependsOn([]Resource{d1, d2}), DependsOn([]Resource{d2, d3}))
	assert.Equal(t, []Resource{d1, d2, d2, d3}, opts.DependsOn)
}

func TestResourceOptionMergingProtect(t *testing.T) {
	// last value wins
	opts := merge(Protect(true), Protect(false))
	assert.Equal(t, false, opts.Protect)
}

func TestResourceOptionMergingDeleteBeforeReplace(t *testing.T) {
	// last value wins
	opts := merge(DeleteBeforeReplace(true), DeleteBeforeReplace(false))
	assert.Equal(t, false, opts.DeleteBeforeReplace)
}

func TestResourceOptionMergingImport(t *testing.T) {
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
