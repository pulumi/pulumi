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
	Array   = []Value
	Map     = map[string]Value
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

// GoValue constrains the set of go values that can be contained inside a Value.
//
// Value can also be a null value.
type GoValue interface {
	bool | float64 | string | // Primitive types
		Array | Map | // Collection types
		Asset | Archive | // Pulumi types
		ResourceReference | // Resource references
		computed | null // marker singletons
}

// New creates a new Value from a GoValue.
func New[T GoValue](goValue T) Value {
	return Value{v: normalize(goValue)}
}

func normalize(goValue any) any {
	switch goValue := goValue.(type) {
	case Array:
		if goValue == nil {
			return nil
		}
	case Map:
		if goValue == nil {
			return nil
		}
	case Asset:
		if goValue == nil {
			return nil
		}
	case Archive:
		if goValue == nil {
			return nil
		}
	case null:
		return nil
	}
	return goValue
}

// Any creates a new Value from a GoValue of unknown type. An error is returned if goValue
// is not a member of GoValue.
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
	case Map:
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
	Computed computed
	// Mark a property as an untyped empty value.
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

func as[T GoValue](v Value) T { return v.v.(T) }

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

func (v Value) AsBool() bool                           { return as[bool](v) }
func (v Value) AsNumber() float64                      { return as[float64](v) }
func (v Value) AsString() string                       { return as[string](v) }
func (v Value) AsArray() Array                         { return as[Array](v) }
func (v Value) AsMap() Map                             { return as[Map](v) }
func (v Value) AsAsset() Asset                         { return as[Asset](v) }
func (v Value) AsArchive() Archive                     { return as[Archive](v) }
func (v Value) AsResourceReference() ResourceReference { return as[ResourceReference](v) }

// Secret returns true if the Value is secret.
//
// It does not check if a contained Value is secret.
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

// WithSecret copies v where secret is true.
func (v Value) WithSecret(isSecret bool) Value {
	v.isSecret = isSecret
	return v
}

// HasComputed returns true if the Value or any nested Value is computed.
func (v Value) HasComputed() bool {
	var hasComputed bool
	v.visit(func(v Value) bool {
		hasComputed = v.IsComputed()
		return !hasComputed
	})
	return hasComputed
}

// Dependencies returns the dependency set of v.
func (v Value) Dependencies() []urn.URN { return v.dependencies }

// Set deps as the v.Dependencies() value of the returned Value.
func (v Value) WithDependencies(deps []urn.URN) Value {
	v.dependencies = deps
	return v
}

// Copy performs a deep copy of the Value.
//
// Caveats:
//
// - Archives copies share underlying asset values.
func (v Value) Copy() Value {
	var dependencies []urn.URN
	if v.dependencies != nil {
		dependencies = make([]urn.URN, len(v.dependencies))
		copy(dependencies, v.dependencies)
	}
	var value any
	switch {
	// Primitive values can just be copied
	case v.IsBool(), v.IsNumber(), v.IsString(),
		v.IsNull(), v.IsComputed():
		value = v.v
	case v.IsArray():
		a := v.AsArray()
		cp := make(Array, len(a))
		for i, v := range a {
			cp[i] = v.Copy()
		}
		value = cp
	case v.IsMap():
		m := v.AsMap()
		cp := make(Map, len(m))
		for k, v := range m {
			cp[k] = v.Copy()
		}
		value = cp
	case v.IsAsset():
		a := v.AsAsset()
		if a == nil {
			value = a
		} else {
			cp := *a
			value = &cp
		}
	case v.IsArchive():
		a := v.AsArchive()
		assets := make(map[string]any, len(a.Assets))
		for k, v := range a.Assets {
			// values are of the any type, and thus cannot be reliably deep
			// copied.
			assets[k] = v
		}
		value = &archive.Archive{
			Sig:    a.Sig,
			Hash:   a.Hash,
			Assets: assets,
			Path:   a.Path,
			URI:    a.URI,
		}
	case v.IsResourceReference():
		ref := v.AsResourceReference()
		value = ResourceReference{
			URN:            ref.URN,
			ID:             ref.ID.Copy(),
			PackageVersion: ref.PackageVersion,
		}
	}

	return Value{
		isSecret:     v.isSecret,
		dependencies: dependencies,
		v:            value,
	}

}

// WithGoValue creates a new Value with the inner value newGoValue.
//
// To set to a null or computed value, pass Null or Computed as newGoValue.
func WithGoValue[T GoValue](value Value, newGoValue T) Value {
	value.v = New(newGoValue).v
	return value
}
