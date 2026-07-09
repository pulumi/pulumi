// Copyright 2016, Pulumi Corporation.
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

package test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func Value(maxDepth int) *rapid.Generator[property.Value] {
	// Build one generator per depth level, sharing the shallower generator
	// between the composite branches. Constructing a fresh sub-generator per
	// branch instead would make construction time exponential in maxDepth.
	value := Primitive()
	for i := 1; i < maxDepth; i++ {
		value = rapid.OneOf(
			Primitive(),
			ArrayOf(value),
			MapOf(value),
			SecretOf(value),
			DependenciesOf(value),
		)
	}
	return value
}

func Primitive() *rapid.Generator[property.Value] {
	return rapid.OneOf(
		String(),
		Bool(),
		Number(),
		Null(),
		Computed(),
		Asset(),
		Archive(),
	)
}

func String() *rapid.Generator[property.Value] {
	return rapid.Map(rapid.String(), property.New[string])
}

func Bool() *rapid.Generator[property.Value] { return rapid.Map(rapid.Bool(), property.New[bool]) }

func Number() *rapid.Generator[property.Value] {
	return rapid.Map(rapid.Float64(), property.New[float64])
}

func Null() *rapid.Generator[property.Value] { return rapid.Just(property.Value{}) }

func Computed() *rapid.Generator[property.Value] { return rapid.Just(property.New(property.Computed)) }

func Asset() *rapid.Generator[property.Value] {
	return rapid.Map(assetGen(), property.New[property.Asset])
}

func Archive() *rapid.Generator[property.Value] {
	return rapid.Map(archiveGen(archiveNestingDepth), property.New[property.Archive])
}

// archiveNestingDepth bounds how deeply generated archives nest within themselves. It is
// independent of the depth budget of [Value], since assets and archives are leaves at the
// [property.Value] level.
const archiveNestingDepth = 3

// assetGen generates raw assets of all three variants: text, path, and URI.
func assetGen() *rapid.Generator[property.Asset] {
	textAsset := rapid.Custom(func(t *rapid.T) property.Asset {
		a, err := asset.FromText(rapid.String().Draw(t, "text"))
		if err != nil {
			panic(err)
		}
		return a
	})
	pathAsset := rapid.Custom(func(t *rapid.T) property.Asset {
		path := rapid.StringN(1, -1, -1).Draw(t, "path")
		return &asset.Asset{Sig: asset.AssetSig, Path: path, Hash: contentHash("asset:path", path)}
	})
	uriAsset := rapid.Custom(func(t *rapid.T) property.Asset {
		uri := rapid.StringN(1, -1, -1).Draw(t, "uri")
		return &asset.Asset{Sig: asset.AssetSig, URI: uri, Hash: contentHash("asset:uri", uri)}
	})
	return rapid.OneOf(textAsset, pathAsset, uriAsset)
}

// archiveGen generates raw archives of all three variants: literal assets, path, and
// URI. Path and URI archives have a nil Assets map; literal archives always have a
// non-nil (possibly empty) Assets map.
func archiveGen(maxDepth int) *rapid.Generator[property.Archive] {
	literalArchive := rapid.Custom(func(t *rapid.T) property.Archive {
		keys := rapid.SliceOfNDistinct(rapid.String(), 0, 4, rapid.ID[string]).Draw(t, "keys")
		assets := make(map[string]any, len(keys))
		for i, k := range keys {
			if maxDepth > 1 && rapid.Bool().Draw(t, fmt.Sprintf("entry %d is archive", i)) {
				assets[k] = archiveGen(maxDepth-1).Draw(t, fmt.Sprintf("sub-archive %d", i))
			} else {
				assets[k] = assetGen().Draw(t, fmt.Sprintf("sub-asset %d", i))
			}
		}
		hashParts := make([]string, 0, 1+2*len(assets))
		hashParts = append(hashParts, "archive:assets")
		for _, k := range slices.Sorted(maps.Keys(assets)) {
			var entryHash string
			switch entry := assets[k].(type) {
			case property.Asset:
				entryHash = entry.Hash
			case property.Archive:
				entryHash = entry.Hash
			}
			hashParts = append(hashParts, k, entryHash)
		}
		return &archive.Archive{Sig: archive.ArchiveSig, Assets: assets, Hash: contentHash(hashParts...)}
	})
	pathArchive := rapid.Custom(func(t *rapid.T) property.Archive {
		path := rapid.StringN(1, -1, -1).Draw(t, "path")
		return &archive.Archive{Sig: archive.ArchiveSig, Path: path, Hash: contentHash("archive:path", path)}
	})
	uriArchive := rapid.Custom(func(t *rapid.T) property.Archive {
		uri := rapid.StringN(1, -1, -1).Draw(t, "uri")
		return &archive.Archive{Sig: archive.ArchiveSig, URI: uri, Hash: contentHash("archive:uri", uri)}
	})
	return rapid.OneOf(literalArchive, pathArchive, uriArchive)
}

// contentHash derives a stand-in hash from the strings that identify a generated asset
// or archive. Path and URI variants cannot compute their real hash without file or
// network I/O, and both [asset.Asset.Equals] and [archive.Archive.Equals] compare only
// hashes — so deriving the hash from content keeps hash-based equality consistent with
// structural equality for generated values. Parts are length-prefixed so distinct part
// lists cannot produce the same digest input.
func contentHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:%s", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func Array(maxDepth int) *rapid.Generator[property.Value] { return ArrayOf(Value(maxDepth - 1)) }

func Map(maxDepth int) *rapid.Generator[property.Value] { return MapOf(Value(maxDepth - 1)) }

func Secret(maxDepth int) *rapid.Generator[property.Value] { return SecretOf(Value(maxDepth - 1)) }

func Dependencies(maxDepth int) *rapid.Generator[property.Value] {
	return DependenciesOf(Value(maxDepth - 1))
}

func ArrayOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return property.New(rapid.SliceOf(value).Draw(t, "V"))
	})
}

func MapOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return property.New(rapid.MapOf(
			rapid.String(),
			value,
		).Draw(t, "V"))
	})
}

func SecretOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return value.Draw(t, "V").WithSecret(true)
	})
}

func DependenciesOf(value *rapid.Generator[property.Value]) *rapid.Generator[property.Value] {
	return rapid.Custom(func(t *rapid.T) property.Value {
		return value.Draw(t, "V").WithDependencies(
			rapid.SliceOfN(URN(), 1, 10).Draw(t, "urns"),
		)
	})
}

// A rapid generator for resource.URN.
//
// Because the github.com/pulumi/pulumi/sdk/v3/go/property does not enforce URN validity,
// we don't enforce it here.
func URN() *rapid.Generator[urn.URN] {
	return rapid.Custom(func(t *rapid.T) urn.URN {
		return urn.URN(rapid.String().Draw(t, "urn-body"))
	})
}
