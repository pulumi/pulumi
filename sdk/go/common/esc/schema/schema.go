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
	"github.com/pulumi/pulumi-cloud-sdk/go/apitype"
)

type Builder interface {
	Schema() *Schema
}

type MapBuilder interface {
	Build() map[string]*Schema
}

type BuilderMap map[string]Builder

func (m BuilderMap) Build() map[string]*Schema {
	s := make(map[string]*Schema, len(m))
	for k, v := range m {
		s[k] = v.Schema()
	}
	return s
}

type SchemaMap map[string]*Schema

func (m SchemaMap) Build() map[string]*Schema {
	return m
}

// Schema is a JSON Schema used to describe the shape of environment values.
type Schema = apitype.EscSchemaSchema

func Never() *Schema {
	return &Schema{Never: true}
}

func Always() *Schema {
	return &Schema{Always: true}
}

func Ref(ref string) *Schema {
	return &Schema{Ref: ref}
}

func AnyOf(anyOf ...Builder) *Schema {
	s := &Schema{}
	return buildAnyOf(s, anyOf)
}

func OneOf(oneOf ...Builder) *Schema {
	s := &Schema{}
	return buildOneOf(s, oneOf)
}

func buildDefs[T Builder](b T, defs map[string]Builder) T {
	s := b.Schema()
	s.Defs = make(map[string]*Schema, len(defs))
	for k, v := range defs {
		s.Defs[k] = v.Schema()
	}
	return b
}

func buildRef[T Builder](b T, ref string) T {
	b.Schema().Ref = ref
	return b
}

func buildAnyOf[T Builder](b T, anyOf []Builder) T {
	s := b.Schema()
	s.AnyOf = make([]*Schema, len(anyOf))
	for i, b := range anyOf {
		s.AnyOf[i] = b.Schema()
	}
	return b
}

func buildOneOf[T Builder](b T, oneOf []Builder) T {
	s := b.Schema()
	s.OneOf = make([]*Schema, len(oneOf))
	for i, b := range oneOf {
		s.OneOf[i] = b.Schema()
	}
	return b
}
