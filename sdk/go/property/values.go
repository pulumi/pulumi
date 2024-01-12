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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type (
	MapKey            tokens.Name
	Array             = []Value
	Map               = map[MapKey]Value
	Asset             = *resource.Asset
	Archive           = *resource.Archive
	ResourceReference = resource.ResourceReference
)

// Value is an imitable representation of a Pulumi value.
//
// It may represent any type in GoValues. In addition, values may be secret or
// computed. It may have resource dependencies.
//
// The zero value of Value is null.
type Value struct {
	isComputed bool
	isSecret   bool

	dependencies []resource.URN // the dependencies associated with this value.

	v any
}

// GoValues constrains the set of go values that can be contained inside a Value.
//
// Value can also be a null value.
type GoValues interface {
	bool | float64 | string | // Primitive types
		Array | Map | // Collection types
		Asset | Archive | // Pulumi types
		ResourceReference // Resource references
}

// Create a new Value from a GoValue.
func Of[T GoValues](goValue T) Value {
	return Value{v: goValue}
}

func is[T GoValues](v Value) bool {
	_, ok := v.v.(T)
	return ok
}

func as[T GoValues](v Value) T { return v.v.(T) }

func (v Value) IsBool() bool              { return is[bool](v) }
func (v Value) IsNumber() bool            { return is[float64](v) }
func (v Value) IsString() bool            { return is[string](v) }
func (v Value) IsArray() bool             { return is[Array](v) }
func (v Value) IsMap() bool               { return is[Map](v) }
func (v Value) IsAsset() bool             { return is[Asset](v) }
func (v Value) IsArchive() bool           { return is[Archive](v) }
func (v Value) IsResourceReference() bool { return is[ResourceReference](v) }
func (v Value) IsNull() bool              { return v.v == nil }

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
func (v Value) WithSecret() Value {
	v.isSecret = true
	return v
}

// WithNotSecret copies v where secret is false.
func (v Value) WithNotSecret() Value {
	v.isSecret = false
	return v
}

// Computed returns true if the Value is computed.
//
// It does not check if a contained Value is computed.
func (v Value) Computed() bool { return v.isComputed }

// HasComputed returns true if the Value or any nested Value is computed.
func (v Value) HasComputed() bool {
	var hasComputed bool
	v.visit(func(v Value) bool {
		hasComputed = v.isComputed
		return !hasComputed
	})
	return hasComputed
}

// WithComputed copies v where computed is true.
func (v Value) WithComputed() Value {
	v.isComputed = true
	return v
}

// WithNotComputed copies v where computed is false.
func (v Value) WithNotComputed() Value {
	v.isComputed = false
	return v
}

// The dependency set of v.
func (v Value) Dependencies() []resource.URN { return v.dependencies }

// Set deps as the v.Dependencies() value of the returned Value.
func (v Value) WithDependencies(deps []resource.URN) Value {
	v.dependencies = deps
	return v
}
