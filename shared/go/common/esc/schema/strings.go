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

type StringBuilder struct {
	s Schema
}

func String() *StringBuilder {
	return &StringBuilder{}
}

func (b *StringBuilder) Ref(ref string) *StringBuilder {
	return buildRef(b, ref)
}

func (b *StringBuilder) AnyOf(anyOf ...Builder) *StringBuilder {
	return buildAnyOf(b, anyOf)
}

func (b *StringBuilder) OneOf(oneOf ...Builder) *StringBuilder {
	return buildOneOf(b, oneOf)
}

func (b *StringBuilder) Const(n string) *StringBuilder {
	b.s.Const = n
	return b
}

func (b *StringBuilder) Enum(vals ...string) *StringBuilder {
	anys := make([]any, len(vals))
	for i, v := range vals {
		anys[i] = v
	}
	b.s.Enum = anys
	return b
}

func (b *StringBuilder) MaxLength(n int) *StringBuilder {
	b.s.MaxLength = json.Number(strconv.FormatInt(int64(n), 10))
	return b
}

func (b *StringBuilder) MinLength(n int) *StringBuilder {
	b.s.MinLength = json.Number(strconv.FormatInt(int64(n), 10))
	return b
}

func (b *StringBuilder) Pattern(pattern string) *StringBuilder {
	b.s.Pattern = pattern
	return b
}

func (b *StringBuilder) Title(title string) *StringBuilder {
	b.s.Title = title
	return b
}

func (b *StringBuilder) Description(description string) *StringBuilder {
	b.s.Description = description
	return b
}

func (b *StringBuilder) Default(n string) *StringBuilder {
	b.s.Default = n
	return b
}

func (b *StringBuilder) Deprecated(deprecated bool) *StringBuilder {
	b.s.Deprecated = deprecated
	return b
}

func (b *StringBuilder) Examples(ns ...string) *StringBuilder {
	vals := make([]any, len(ns))
	for i, n := range ns {
		vals[i] = n
	}
	b.s.Examples = vals
	return b
}

func (b *StringBuilder) Schema() *Schema {
	b.s.Type = "string"
	return &b.s
}
