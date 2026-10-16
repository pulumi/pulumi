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

type BooleanBuilder struct {
	s Schema
}

func Boolean() *BooleanBuilder {
	return &BooleanBuilder{}
}

func (b *BooleanBuilder) Ref(ref string) *BooleanBuilder {
	return buildRef(b, ref)
}

func (b *BooleanBuilder) AnyOf(anyOf ...Builder) *BooleanBuilder {
	return buildAnyOf(b, anyOf)
}

func (b *BooleanBuilder) OneOf(oneOf ...Builder) *BooleanBuilder {
	return buildOneOf(b, oneOf)
}

func (b *BooleanBuilder) Const(v bool) *BooleanBuilder {
	b.s.Const = v
	return b
}

func (b *BooleanBuilder) Title(title string) *BooleanBuilder {
	b.s.Title = title
	return b
}

func (b *BooleanBuilder) Description(description string) *BooleanBuilder {
	b.s.Description = description
	return b
}

func (b *BooleanBuilder) Default(v bool) *BooleanBuilder {
	b.s.Default = v
	return b
}

func (b *BooleanBuilder) Deprecated(deprecated bool) *BooleanBuilder {
	b.s.Deprecated = deprecated
	return b
}

func (b *BooleanBuilder) Schema() *Schema {
	b.s.Type = "boolean"
	return &b.s
}
