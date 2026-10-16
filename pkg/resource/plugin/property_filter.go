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
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// FilterOptions configures the pre-marshal / post-unmarshal filtering applied to a
// [property.Map] before handing it to propertyrpc. The filter is a drop-in replacement for
// the behaviour that used to live inside MarshalOptions in sdk/go/common/resource/plugin.
//
// Options that only affect the wire encoding itself (byte-string envelopes, asset
// serialisation) stay with the marshal step; this type only expresses tree-shape
// transformations and validation.
type FilterOptions struct {
	// KeepUnknowns retains computed (unknown) values. When false, map entries whose
	// value is computed are dropped; computed values inside arrays become null.
	KeepUnknowns bool
	// RejectUnknowns returns an error if any computed value is encountered. Takes
	// precedence over KeepUnknowns.
	RejectUnknowns bool
	// KeepSecrets retains the secret marker on values. When false, the marker is
	// stripped (the underlying value is preserved).
	KeepSecrets bool
	// KeepResources retains resource references. When false, each reference is
	// replaced by its ID (if present) or its URN string.
	KeepResources bool
	// KeepDependencies retains dependency metadata on values. When false, the
	// dependency set is cleared (secret markers are preserved).
	KeepDependencies bool
	// SkipNulls omits map entries whose value is null.
	SkipNulls bool
	// SkipInternalKeys omits map entries whose key begins with "__".
	SkipInternalKeys bool
	// RejectAssets returns an error if any asset or archive value is encountered.
	RejectAssets bool
	// ElideAssetContents replaces each asset and archive with one that carries only
	// its hash, discarding Text/Path/URI/Assets. Takes precedence over
	// WorkingDirectory.
	ElideAssetContents bool
	// WorkingDirectory, if non-empty, populates missing Hash fields on assets and
	// archives. Relative asset/archive paths are resolved against this directory.
	// Pass "." to compute with the current working directory.
	WorkingDirectory string
	// RejectNonUTF8Strings indicates that the receiver will reject strings containing
	// non-UTF-8 bytes. When false, such strings are accepted.
	RejectNonUTF8Strings bool
}

// pathString renders a [property.Path] in its human-readable form (e.g. "foo[2].bar").
// The empty path is rendered as "<root>".
func pathString(p property.Path) string {
	b, err := p.MarshalText()
	// MarshalText errors on empty path
	if err != nil || len(b) == 0 {
		return "<root>"
	}
	return string(b)
}

// FilterProperties returns a new [property.Map] with the configured filtering applied.
// The input is not mutated.
func FilterProperties(m property.Map, opts FilterOptions) (property.Map, error) {
	return filterProperties(m, opts, property.Path{})
}

func filterProperties(m property.Map, opts FilterOptions, path property.Path) (property.Map, error) {
	out := make(map[string]property.Value, m.Len())
	for k, v := range m.All {
		if opts.SkipInternalKeys && strings.HasPrefix(k, "__") {
			continue
		}
		nv, keep, err := filterValue(v, opts, path.AppendKey(k))
		if err != nil {
			return property.Map{}, err
		}
		if !keep {
			continue
		}
		if opts.SkipNulls && nv.IsNull() {
			continue
		}
		out[k] = nv
	}
	return property.NewMap(out), nil
}

// FilterPropertyValue applies the filter to a single value. The second return value is
// false if the value should be omitted entirely (e.g. a computed value with
// KeepUnknowns=false appearing at the root of a filter call).
func FilterPropertyValue(v property.Value, opts FilterOptions) (property.Value, bool, error) {
	return filterValue(v, opts, property.Path{})
}

func filterValue(v property.Value, opts FilterOptions, path property.Path) (property.Value, bool, error) {
	// Dependencies (output-value metadata).
	if !opts.KeepDependencies && len(v.Dependencies()) > 0 {
		v = v.WithDependencies(nil)
	}

	// Secret marker.
	if !opts.KeepSecrets && v.Secret() {
		v = v.WithSecret(false)
	}

	switch {
	case v.IsComputed():
		if opts.RejectUnknowns {
			return property.Value{}, false, fmt.Errorf("unexpected unknown value at %s", pathString(path))
		}
		if !opts.KeepUnknowns {
			return property.Value{}, false, nil
		}
		return v, true, nil

	case v.IsString():
		if opts.RejectNonUTF8Strings && !utf8.ValidString(v.AsString()) {
			return property.Value{}, false, fmt.Errorf(
				"the value of property %s is a string that contains non-UTF8 bytes, "+
					"which the receiver does not support", pathString(path))
		}
		return v, true, nil

	case v.IsAsset():
		if opts.RejectAssets {
			return property.Value{}, false, fmt.Errorf("unexpected asset value at %s", pathString(path))
		}
		a := v.AsAsset()
		if opts.ElideAssetContents {
			return property.WithGoValue(v, &asset.Asset{Hash: a.Hash}), true, nil
		}
		if opts.WorkingDirectory != "" && a.Hash == "" {
			err := a.EnsureHashWithWD(opts.WorkingDirectory)
			if err != nil {
				return property.Value{}, false, fmt.Errorf("failed to compute asset hash at %s: %w", pathString(path), err)
			}
			return property.WithGoValue(v, a), true, nil
		}
		return v, true, nil

	case v.IsArchive():
		if opts.RejectAssets {
			return property.Value{}, false, fmt.Errorf("unexpected archive value at %s", pathString(path))
		}
		a := v.AsArchive()
		if opts.ElideAssetContents {
			return property.WithGoValue(v, &archive.Archive{Hash: a.Hash}), true, nil
		}
		if opts.WorkingDirectory != "" && a.Hash == "" {
			err := a.EnsureHashWithWD(opts.WorkingDirectory)
			if err != nil {
				return property.Value{}, false, fmt.Errorf("failed to compute archive hash at %s: %w", pathString(path), err)
			}
			return property.WithGoValue(v, a), true, nil
		}
		return v, true, nil

	case v.IsResourceReference():
		if opts.KeepResources {
			return v, true, nil
		}
		ref := v.AsResourceReference()
		// Match the legacy MarshalProperties behaviour: if the reference carries an ID
		// (string or computed) use that in place of the reference, otherwise fall back
		// to the URN. Re-run the filter on the substituted value so flags like
		// KeepUnknowns and AcceptsByteString still apply.
		var replacement property.Value
		if !ref.ID.IsNull() {
			replacement = ref.ID
		} else {
			replacement = property.New(string(ref.URN))
		}
		replacement = replacement.WithSecret(v.Secret()).WithDependencies(v.Dependencies())
		return filterValue(replacement, opts, path)

	case v.IsArray():
		arr := v.AsArray()
		out := make([]property.Value, 0, arr.Len())
		for i, elem := range arr.All {
			nv, keep, err := filterValue(elem, opts, path.AppendIndex(i))
			if err != nil {
				return property.Value{}, false, err
			}
			if !keep {
				// Preserve array positions by substituting null for dropped elements.
				out = append(out, property.New(property.Null))
				continue
			}
			out = append(out, nv)
		}
		return property.WithGoValue(v, property.NewArray(out)), true, nil

	case v.IsMap():
		m := v.AsMap()
		inner, err := filterProperties(m, opts, path)
		if err != nil {
			return property.Value{}, false, err
		}
		return property.WithGoValue(v, inner), true, nil

	default:
		return v, true, nil
	}
}
