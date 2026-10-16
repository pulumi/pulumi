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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/shared/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/shared/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/shared/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/shared/v3/go/property"
	propertytest "github.com/pulumi/pulumi/shared/v3/go/property/testing"
)

// keepAll is the identity filter: everything through, byte strings accepted.
var keepAll = FilterOptions{
	KeepUnknowns:     true,
	KeepSecrets:      true,
	KeepResources:    true,
	KeepDependencies: true,
}

func hasKey(m property.Map, k string) bool {
	_, ok := m.GetOk(k)
	return ok
}

func filterMap(t *testing.T, m property.Map, opts FilterOptions) property.Map {
	t.Helper()
	out, err := FilterProperties(m, opts)
	require.NoError(t, err)
	return out
}

func TestFilterIdentity(t *testing.T) {
	t.Parallel()

	m := property.NewMap(map[string]property.Value{
		"s":    property.New("hello"),
		"n":    property.New(float64(1)),
		"b":    property.New(true),
		"null": property.New(property.Null),
		"arr":  property.New([]property.Value{property.New("x")}),
		"map":  property.New(map[string]property.Value{"k": property.New("v")}),
	})

	assert.Equal(t, m, filterMap(t, m, keepAll))
}

func TestFilterKeepUnknowns(t *testing.T) {
	t.Parallel()

	m := property.NewMap(map[string]property.Value{
		"known":   property.New("v"),
		"unknown": property.New(property.Computed),
		"arr":     property.New([]property.Value{property.New("a"), property.New(property.Computed)}),
	})

	// KeepUnknowns=false drops computed map entries and nulls them in arrays.
	out := filterMap(t, m, FilterOptions{KeepSecrets: true, KeepResources: true})
	assert.False(t, hasKey(out, "unknown"))
	assert.True(t, hasKey(out, "known"))
	arr := out.Get("arr").AsArray()
	require.Equal(t, 2, arr.Len())
	assert.True(t, arr.Get(1).IsNull())

	// KeepUnknowns=true preserves them.
	out = filterMap(t, m, keepAll)
	assert.True(t, out.Get("unknown").IsComputed())
	assert.True(t, out.Get("arr").AsArray().Get(1).IsComputed())
}

func TestFilterRejectUnknowns(t *testing.T) {
	t.Parallel()

	m := property.NewMap(map[string]property.Value{
		"unknown": property.New(property.Computed),
	})
	opts := keepAll
	opts.RejectUnknowns = true
	_, err := FilterProperties(m, opts)
	assert.EqualError(t, err, `unexpected unknown value at unknown`)

	// Nested computed is also rejected and the path points at the array index.
	m = property.NewMap(map[string]property.Value{
		"arr": property.New([]property.Value{property.New("ok"), property.New(property.Computed)}),
	})
	_, err = FilterProperties(m, opts)
	assert.EqualError(t, err, `unexpected unknown value at arr[1]`)

	// Deep nesting: object inside array inside map.
	m = property.NewMap(map[string]property.Value{
		"outer": property.New([]property.Value{
			property.New(map[string]property.Value{"inner": property.New(property.Computed)}),
		}),
	})
	_, err = FilterProperties(m, opts)
	assert.EqualError(t, err, `unexpected unknown value at outer[0].inner`)
}

func TestFilterKeepSecrets(t *testing.T) {
	t.Parallel()

	m := property.NewMap(map[string]property.Value{
		"s": property.New("secret-value").WithSecret(true),
	})

	// KeepSecrets=false strips the marker, retains the value.
	out := filterMap(t, m, FilterOptions{KeepUnknowns: true, KeepResources: true})
	v := out.Get("s")
	assert.False(t, v.Secret())
	assert.Equal(t, "secret-value", v.AsString())

	// KeepSecrets=true preserves it.
	out = filterMap(t, m, keepAll)
	assert.True(t, out.Get("s").Secret())
}

func TestFilterKeepResources(t *testing.T) {
	t.Parallel()

	refWithID := property.ResourceReference{
		URN: urn.URN("urn:pulumi:stack::proj::type::name"),
		ID:  property.New("abc123"),
	}
	refWithoutID := property.ResourceReference{
		URN: urn.URN("urn:pulumi:stack::proj::type::other"),
	}
	refComputedID := property.ResourceReference{
		URN: urn.URN("urn:pulumi:stack::proj::type::computed"),
		ID:  property.New(property.Computed),
	}

	m := property.NewMap(map[string]property.Value{
		"with":    property.New(refWithID),
		"without": property.New(refWithoutID),
		"comp":    property.New(refComputedID),
	})

	// KeepResources=true round-trips.
	out := filterMap(t, m, keepAll)
	assert.True(t, out.Get("with").IsResourceReference())

	// KeepResources=false: ID when present, URN otherwise.
	opts := FilterOptions{KeepUnknowns: true, KeepSecrets: true}
	out = filterMap(t, m, opts)
	assert.Equal(t, "abc123", out.Get("with").AsString())
	assert.Equal(t, string(refWithoutID.URN), out.Get("without").AsString())
	// Computed ID preserved as computed value.
	assert.True(t, out.Get("comp").IsComputed())
}

func TestFilterKeepResourcesPreservesSecretAndDeps(t *testing.T) {
	t.Parallel()

	ref := property.ResourceReference{
		URN: urn.URN("urn:pulumi:stack::proj::type::name"),
		ID:  property.New("abc"),
	}
	deps := []urn.URN{"urn:pulumi:stack::proj::type::dep"}
	m := property.NewMap(map[string]property.Value{
		"r": property.New(ref).WithSecret(true).WithDependencies(deps),
	})

	opts := FilterOptions{KeepUnknowns: true, KeepSecrets: true, KeepDependencies: true}
	out := filterMap(t, m, opts)
	v := out.Get("r")
	assert.Equal(t, "abc", v.AsString())
	assert.True(t, v.Secret())
	assert.Equal(t, deps, v.Dependencies())
}

func TestFilterKeepOutputValues(t *testing.T) {
	t.Parallel()

	deps := []urn.URN{"urn:pulumi:stack::proj::type::dep"}
	m := property.NewMap(map[string]property.Value{
		"v": property.New("x").WithDependencies(deps),
	})

	// KeepOutputValues=false strips dependencies.
	opts := FilterOptions{KeepUnknowns: true, KeepSecrets: true, KeepResources: true}
	out := filterMap(t, m, opts)
	assert.Empty(t, out.Get("v").Dependencies())

	// KeepOutputValues=true preserves them.
	out = filterMap(t, m, keepAll)
	assert.Equal(t, deps, out.Get("v").Dependencies())
}

func TestFilterSkipNulls(t *testing.T) {
	t.Parallel()

	m := property.NewMap(map[string]property.Value{
		"a": property.New("v"),
		"b": property.New(property.Null),
	})

	opts := keepAll
	opts.SkipNulls = true
	out := filterMap(t, m, opts)
	assert.True(t, hasKey(out, "a"))
	assert.False(t, hasKey(out, "b"))
}

func TestFilterSkipInternalKeys(t *testing.T) {
	t.Parallel()

	m := property.NewMap(map[string]property.Value{
		"public":  property.New("v"),
		"__meta":  property.New("hidden"),
		"__other": property.New("hidden"),
	})

	opts := keepAll
	opts.SkipInternalKeys = true
	out := filterMap(t, m, opts)
	assert.True(t, hasKey(out, "public"))
	assert.False(t, hasKey(out, "__meta"))
	assert.False(t, hasKey(out, "__other"))
}

func TestFilterRejectAssets(t *testing.T) {
	t.Parallel()

	a, err := asset.FromText("hello")
	require.NoError(t, err)
	m := property.NewMap(map[string]property.Value{"top": property.New(a)})

	opts := keepAll
	opts.RejectAssets = true
	_, err = FilterProperties(m, opts)
	assert.EqualError(t, err, `unexpected asset value at top`)

	// Archive at a nested path.
	arc, err := archive.FromAssets(map[string]any{"f": a})
	require.NoError(t, err)
	m = property.NewMap(map[string]property.Value{
		"outer": property.New([]property.Value{property.New(arc)}),
	})
	_, err = FilterProperties(m, opts)
	assert.EqualError(t, err, `unexpected archive value at outer[0]`)
}

func TestFilterElideAssetContents(t *testing.T) {
	t.Parallel()

	a, err := asset.FromText("hello")
	require.NoError(t, err)
	arc, err := archive.FromAssets(map[string]any{"f": a})
	require.NoError(t, err)

	m := property.NewMap(map[string]property.Value{
		"a": property.New(a),
		"r": property.New(arc),
	})

	opts := keepAll
	opts.ElideAssetContents = true
	out := filterMap(t, m, opts)

	elidedA := out.Get("a").AsAsset()
	assert.Equal(t, a.Hash, elidedA.Hash)
	assert.Empty(t, elidedA.Text)
	assert.Empty(t, elidedA.Path)
	assert.Empty(t, elidedA.URI)

	elidedR := out.Get("r").AsArchive()
	assert.Equal(t, arc.Hash, elidedR.Hash)
	assert.Empty(t, elidedR.Assets)
	assert.Empty(t, elidedR.Path)
	assert.Empty(t, elidedR.URI)
}

func TestFilterAcceptsByteString(t *testing.T) {
	t.Parallel()

	invalid := "\xff\xfe"
	m := property.NewMap(map[string]property.Value{
		"ok":  property.New("hello"),
		"bad": property.New(invalid),
	})

	// AcceptsByteString=true: no error.
	_, err := FilterProperties(m, keepAll)
	require.NoError(t, err)

	// RejectNonUTF8Strings=true: error pointing at the offending key.
	opts := keepAll
	opts.RejectNonUTF8Strings = true
	_, err = FilterProperties(m, opts)
	assert.EqualError(t, err,
		`the value of property bad is a string that contains non-UTF8 bytes, `+
			`which the receiver does not support`)

	// Nested path through an array.
	nested := property.NewMap(map[string]property.Value{
		"outer": property.New([]property.Value{property.New(invalid)}),
	})
	_, err = FilterProperties(nested, opts)
	assert.EqualError(t, err,
		`the value of property outer[0] is a string that contains non-UTF8 bytes, `+
			`which the receiver does not support`)
}

func TestFilterWorkingDirectoryFillsMissingHashes(t *testing.T) {
	t.Parallel()

	a, err := asset.FromText("hello")
	require.NoError(t, err)
	a.Hash = ""

	m := property.NewMap(map[string]property.Value{
		"a":   property.New(a),
		"arr": property.New([]property.Value{property.New(a)}),
	})

	opts := keepAll
	opts.WorkingDirectory = "."
	out := filterMap(t, m, opts)

	topHash := out.Get("a").AsAsset().Hash
	assert.NotEmpty(t, topHash)
	nestedHash := out.Get("arr").AsArray().Get(0).AsAsset().Hash
	assert.Equal(t, topHash, nestedHash)

	// The input is not mutated.
	assert.Empty(t, m.Get("a").AsAsset().Hash)
}

func TestFilterWorkingDirectoryPreservesExistingHashes(t *testing.T) {
	t.Parallel()

	a, err := asset.FromText("hello")
	require.NoError(t, err)
	original := a.Hash
	require.NotEmpty(t, original)

	m := property.NewMap(map[string]property.Value{"a": property.New(a)})
	opts := keepAll
	opts.WorkingDirectory = "."
	out := filterMap(t, m, opts)
	assert.Equal(t, original, out.Get("a").AsAsset().Hash)
}

func TestFilterElideBeatsWorkingDirectory(t *testing.T) {
	t.Parallel()

	a, err := asset.FromText("hello")
	require.NoError(t, err)
	original := a.Hash

	m := property.NewMap(map[string]property.Value{"a": property.New(a)})
	opts := keepAll
	opts.ElideAssetContents = true
	opts.WorkingDirectory = "."
	out := filterMap(t, m, opts)

	elided := out.Get("a").AsAsset()
	assert.Equal(t, original, elided.Hash)
	assert.Empty(t, elided.Text)
}

// TestFilterIdentityRapid: with keepAll the filter is a no-op on arbitrary trees.
// AcceptsByteString=true suppresses the only validation path, so this is a pure
// structural identity check modulo WithGoValue wrapping (which preserves equality).
func TestFilterIdentityRapid(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		m := propertytest.Map(4).Draw(t, "map")
		out, err := FilterProperties(m, keepAll)
		require.NoError(t, err)
		require.True(t, m.Equals(out))
	})
}
