// Copyright 2023, Pulumi Corporation.
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

package schema

import "encoding/json"

type NumberBuilder struct {
	s Schema
}

func Number() *NumberBuilder {
	return &NumberBuilder{}
}

func (b *NumberBuilder) Ref(ref string) *NumberBuilder {
	return buildRef(b, ref)
}

func (b *NumberBuilder) AnyOf(anyOf ...Builder) *NumberBuilder {
	return buildAnyOf(b, anyOf)
}

func (b *NumberBuilder) OneOf(oneOf ...Builder) *NumberBuilder {
	return buildOneOf(b, oneOf)
}

func (b *NumberBuilder) Const(n json.Number) *NumberBuilder {
	b.s.Const = n
	return b
}

func (b *NumberBuilder) Enum(vals ...json.Number) *NumberBuilder {
	anys := make([]any, len(vals))
	for i, v := range vals {
		anys[i] = v
	}
	b.s.Enum = anys
	return b
}

func (b *NumberBuilder) MultipleOf(n json.Number) *NumberBuilder {
	b.s.MultipleOf = n
	return b
}

func (b *NumberBuilder) Maximum(n json.Number) *NumberBuilder {
	b.s.Maximum = n
	return b
}

func (b *NumberBuilder) ExclusiveMaximum(n json.Number) *NumberBuilder {
	b.s.ExclusiveMaximum = n
	return b
}

func (b *NumberBuilder) Minimum(n json.Number) *NumberBuilder {
	b.s.Minimum = n
	return b
}

func (b *NumberBuilder) ExclusiveMinimum(n json.Number) *NumberBuilder {
	b.s.ExclusiveMinimum = n
	return b
}

func (b *NumberBuilder) Title(title string) *NumberBuilder {
	b.s.Title = title
	return b
}

func (b *NumberBuilder) Description(description string) *NumberBuilder {
	b.s.Description = description
	return b
}

func (b *NumberBuilder) Default(n json.Number) *NumberBuilder {
	b.s.Default = n
	return b
}

func (b *NumberBuilder) Deprecated(deprecated bool) *NumberBuilder {
	b.s.Deprecated = deprecated
	return b
}

func (b *NumberBuilder) Examples(ns ...json.Number) *NumberBuilder {
	vals := make([]any, len(ns))
	for i, n := range ns {
		vals[i] = n
	}
	b.s.Examples = vals
	return b
}

func (b *NumberBuilder) Schema() *Schema {
	b.s.Type = "number"
	return &b.s
}
