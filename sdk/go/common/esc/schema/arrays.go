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

import (
	"encoding/json"
	"strconv"
)

type ArrayBuilder struct {
	s Schema
}

func Array() *ArrayBuilder {
	return &ArrayBuilder{}
}

func Tuple(prefixItems ...Builder) *ArrayBuilder {
	return Array().PrefixItems(prefixItems...).Items(Never())
}

func (b *ArrayBuilder) Defs(defs map[string]Builder) *ArrayBuilder {
	return buildDefs(b, defs)
}

func (b *ArrayBuilder) Ref(ref string) *ArrayBuilder {
	return buildRef(b, ref)
}

func (b *ArrayBuilder) AnyOf(anyOf ...Builder) *ArrayBuilder {
	return buildAnyOf(b, anyOf)
}

func (b *ArrayBuilder) OneOf(oneOf ...Builder) *ArrayBuilder {
	return buildOneOf(b, oneOf)
}

func (b *ArrayBuilder) PrefixItems(prefixItems ...Builder) *ArrayBuilder {
	b.s.PrefixItems = make([]*Schema, len(prefixItems))
	for i, e := range prefixItems {
		b.s.PrefixItems[i] = e.Schema()
	}
	return b
}

func (b *ArrayBuilder) Items(items Builder) *ArrayBuilder {
	b.s.Items = items.Schema()
	return b
}

func (b *ArrayBuilder) MinItems(n int) *ArrayBuilder {
	b.s.MinItems = json.Number(strconv.FormatInt(int64(n), 10))
	return b
}

func (b *ArrayBuilder) MaxItems(n int) *ArrayBuilder {
	b.s.MaxItems = json.Number(strconv.FormatInt(int64(n), 10))
	return b
}

func (b *ArrayBuilder) UniqueItems(v bool) *ArrayBuilder {
	b.s.UniqueItems = v
	return b
}

func (b *ArrayBuilder) Title(title string) *ArrayBuilder {
	b.s.Title = title
	return b
}

func (b *ArrayBuilder) Description(description string) *ArrayBuilder {
	b.s.Description = description
	return b
}

func (b *ArrayBuilder) Default(v []any) *ArrayBuilder {
	b.s.Default = v
	return b
}

func (b *ArrayBuilder) Deprecated(deprecated bool) *ArrayBuilder {
	b.s.Deprecated = deprecated
	return b
}

func (b *ArrayBuilder) Examples(vals ...[]any) *ArrayBuilder {
	anys := make([]any, len(vals))
	for i, v := range vals {
		anys[i] = v
	}
	b.s.Examples = anys
	return b
}

func (b *ArrayBuilder) Schema() *Schema {
	b.s.Type = "array"
	return &b.s
}
