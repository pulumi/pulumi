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
	"sort"
	"strconv"

	"golang.org/x/exp/maps"
)

type ObjectBuilder struct {
	s Schema
}

func Object() *ObjectBuilder {
	return &ObjectBuilder{}
}

func Record(m map[string]Builder) *ObjectBuilder {
	names := maps.Keys(m)
	sort.Strings(names)

	return Object().Properties(m).Required(names...)
}

func (b *ObjectBuilder) Defs(defs map[string]Builder) *ObjectBuilder {
	return buildDefs(b, defs)
}

func (b *ObjectBuilder) Ref(ref string) *ObjectBuilder {
	return buildRef(b, ref)
}

func (b *ObjectBuilder) AnyOf(anyOf ...Builder) *ObjectBuilder {
	return buildAnyOf(b, anyOf)
}

func (b *ObjectBuilder) OneOf(oneOf ...Builder) *ObjectBuilder {
	return buildOneOf(b, oneOf)
}

func (b *ObjectBuilder) Properties(m map[string]Builder) *ObjectBuilder {
	b.s.Properties = make(map[string]*Schema, len(m))
	for k, v := range m {
		b.s.Properties[k] = v.Schema()
	}
	return b
}

func (b *ObjectBuilder) AdditionalProperties(s Builder) *ObjectBuilder {
	b.s.AdditionalProperties = s.Schema()
	return b
}

func (b *ObjectBuilder) MinProperties(n int) *ObjectBuilder {
	b.s.MinProperties = json.Number(strconv.FormatInt(int64(n), 10))
	return b
}

func (b *ObjectBuilder) MaxProperties(n int) *ObjectBuilder {
	b.s.MaxProperties = json.Number(strconv.FormatInt(int64(n), 10))
	return b
}

func (b *ObjectBuilder) Required(names ...string) *ObjectBuilder {
	b.s.Required = names
	return b
}

func (b *ObjectBuilder) DependentRequired(names map[string][]string) *ObjectBuilder {
	b.s.DependentRequired = names
	return b
}

func (b *ObjectBuilder) Title(title string) *ObjectBuilder {
	b.s.Title = title
	return b
}

func (b *ObjectBuilder) Description(description string) *ObjectBuilder {
	b.s.Description = description
	return b
}

func (b *ObjectBuilder) Default(v map[string]any) *ObjectBuilder {
	b.s.Default = v
	return b
}

func (b *ObjectBuilder) Deprecated(deprecated bool) *ObjectBuilder {
	b.s.Deprecated = deprecated
	return b
}

func (b *ObjectBuilder) Examples(vals ...map[string]any) *ObjectBuilder {
	anys := make([]any, len(vals))
	for i, v := range vals {
		anys[i] = v
	}
	b.s.Examples = anys
	return b
}

func (b *ObjectBuilder) Schema() *Schema {
	b.s.Type = "object"
	return &b.s
}
