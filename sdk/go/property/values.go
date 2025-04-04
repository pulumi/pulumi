// Copyright 2016-2024, Pulumi Corporation.
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

// The Pulumi value system (formerly resource.PropertyValue)
package property

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
)

type (
	Asset   = *asset.Asset
	Archive = *archive.Archive
)

// Value is an imitable representation of a Pulumi value.
//
// It may represent any type in GoValue. In addition, values may be secret or
// computed. It may have resource dependencies.
//
// The zero value of Value is null.
type Value struct {
	isSecret bool

	dependencies []urn.URN // the dependencies associated with this value.

	// The inner go value for the Value.
	//
	// Note: null{} is not a valid value for v. null{} should be normalized to nil
	// during creation, so that the zero value of Value is bit for bit equivalent to
	// `New(Null)`.
	v any
}

// GoValue defines the set of go values that can be contained inside a [Value].
//
// Value can also be a null value.
type GoValue interface {
	bool | float64 | string | // Primitive types
		Map | map[string]Value | // Map types
		Array | []Value | // Array types
		Asset | Archive | // Pulumi types
		ResourceReference | // Resource references
		computed | null // marker singletons
}

// New creates a new Value from a GoValue.
//
// To create a new value from an unknown type, use [Any].
func New[T GoValue](goValue T) Value {
	return Value{v: normalize(goValue)}
}

func normalize(goValue any) any {
	switch goValue := goValue.(type) {
	case map[string]Value:
		if goValue == nil {
			return nil
		}
		return NewMap(goValue)
	case []Value:
		if goValue == nil {
			return nil
		}
		return NewArray(goValue)
	case Archive:
		if goValue == nil {
			return nil
		}
		return copyArchive(goValue)
	case Asset:
		if goValue == nil {
			return nil
		}
		return copyAsset(goValue)
	case null:
		return nil
	}
	return goValue
}

// Any creates a new [Value] from a [GoValue] of unknown type. An error is returned if
// goValue is not a member of [GoValue].
func Any(goValue any) (Value, error) {
	switch goValue := goValue.(type) {
	case bool:
		return New(goValue), nil
	case float64:
		return New(goValue), nil
	case string:
		return New(goValue), nil
	case Array:
		return New(goValue), nil
	case []Value:
		return New(goValue), nil
	case Map:
		return New(goValue), nil
	case map[string]Value:
		return New(goValue), nil
	case Asset:
		return New(goValue), nil
	case Archive:
		return New(goValue), nil
	case ResourceReference:
		return New(goValue), nil
	case computed:
		return New(goValue), nil
	case nil, null:
		return Value{}, nil
	default:
		return Value{}, fmt.Errorf("invalid type: %s of type %[1]T", goValue)
	}
}

// Computed and Null are marker values of distinct singleton types.
//
// Because the type of the variable is a singleton, it is not possible to mutate these
// values (there is no other value to mutate to).
var (
	// Mark a property as an untyped computed value.
	//
	//	value := property.New(property.Computed)
	//
	// Computed can also be used to mark a [Value] as computed without changing other
	// markers.
	//
	//	value := property.WithValue(maybeSecretValue, property.Computed)
	Computed computed
	// Mark a property as an untyped null value.
	//
	//	value := property.New(property.Null)
	//
	// Null can also be used to mark a [Value] as null without changing other
	// markers.
	//
	//	value := property.WithValue(maybeSecretValue, property.Null)
	//
	// [Value]s can be null, and a null value *is not* equivalent to the absence of a
	// value.
	Null null
)

// Singleton marker types.
//
// These types are intentionally private. Users should instead use the available exported
// values.
type (
	computed struct{}
	null     struct{}
)

func is[T GoValue](v Value) bool {
	_, ok := v.v.(T)
	return ok
}

func asMut[T GoValue](v Value) T { return v.v.(T) }

func (v Value) IsBool() bool              { return is[bool](v) }
func (v Value) IsNumber() bool            { return is[float64](v) }
func (v Value) IsString() bool            { return is[string](v) }
func (v Value) IsArray() bool             { return is[Array](v) }
func (v Value) IsMap() bool               { return is[Map](v) }
func (v Value) IsAsset() bool             { return is[Asset](v) }
func (v Value) IsArchive() bool           { return is[Archive](v) }
func (v Value) IsResourceReference() bool { return is[ResourceReference](v) }
func (v Value) IsNull() bool              { return v.v == nil }
func (v Value) IsComputed() bool          { return is[computed](v) }

// Copy by value types don't distinguish between mutable and non-mutable copies.

func (v Value) AsBool() bool                           { return asMut[bool](v) }
func (v Value) AsNumber() float64                      { return asMut[float64](v) }
func (v Value) AsString() string                       { return asMut[string](v) }
func (v Value) AsResourceReference() ResourceReference { return asMut[ResourceReference](v) }
func (v Value) AsAsset() Asset                         { return copyAsset(asMut[Asset](v)) }
func (v Value) AsArchive() Archive                     { return copyArchive(asMut[Archive](v)) }
func (v Value) AsArray() Array                         { return asMut[Array](v) }
func (v Value) AsMap() Map                             { return asMut[Map](v) }

// copyAsset peforms a deep copy of an asset.
func copyAsset(a Asset) Asset {
	return &asset.Asset{
		Sig:  a.Sig,
		Hash: a.Hash,
		Text: a.Text,
		Path: a.Path,
		URI:  a.URI,
	}
}

func copyArchive(a Archive) Archive {
	assets := make(map[string]any, len(a.Assets))
	for k, v := range a.Assets {
		// TODO: These values are not actually of any type, and need to be copied
		// correctly.
		//
		// values are of the any type, and thus cannot be reliably deep
		// copied.
		assets[k] = v
	}
	return &archive.Archive{
		Sig:    a.Sig,
		Hash:   a.Hash,
		Assets: assets,
		Path:   a.Path,
		URI:    a.URI,
	}
}

// as*Mut act as interior escapes

func (v Value) asAssetMut() Asset     { return asMut[Asset](v) }
func (v Value) asArchiveMut() Archive { return asMut[Archive](v) }

// Secret returns true if the [Value] is secret.
//
// It does not check if there are nested values that are secret. To recursively check if
// the [Value] contains a secret, use [Value.HasSecrets].
func (v Value) Secret() bool { return v.isSecret }

// HasSecrets returns true if the Value or any nested Value is secret.
func (v Value) HasSecrets() bool {
	var hasSecret bool
	v.visit(func(v Value) bool {
		hasSecret = v.isSecret
		return !hasSecret
	})
	return hasSecret
}

// WithSecret produces a new [Value] identical to it's receiver except that it's secret
// market is set to isSecret.
func (v Value) WithSecret(isSecret bool) Value {
	v.isSecret = isSecret
	return v
}

// HasComputed returns true if the Value or any nested Value is computed.
//
// To check if the receiver is itself computed, use [Value.IsComputed].
func (v Value) HasComputed() bool {
	var hasComputed bool
	v.visit(func(v Value) bool {
		hasComputed = v.IsComputed()
		return !hasComputed
	})
	return hasComputed
}

// Dependencies returns the dependency set of v.
//
// To set the dependencies of a value, use [Value.WithDependencies].
func (v Value) Dependencies() []urn.URN {
	// Create a copy of v.dependencies to keep v immutable.
	cp := make([]urn.URN, len(v.dependencies))
	copy(cp, v.dependencies)
	return cp
}

// WithDependencies returns a new value identical to the receiver, except that it has as
// it's dependencies the passed in value.
func (v Value) WithDependencies(dependencies []urn.URN) Value {
	// Create a copy of dependencies to keep v immutable.
	//
	// We don't want exiting references to dependencies to be able to effect
	// v.dependencies.
	v.dependencies = copyArray(dependencies)
	return v
}

// WithGoValue creates a new Value with the inner value newGoValue.
//
// To set a [Value] to a null or computed value, pass [Null] or [Computed] as the new
// value.
func WithGoValue[T GoValue](value Value, newGoValue T) Value {
	value.v = normalize(newGoValue)
	return value
}
