// Copyright 2023-2024, Pulumi Corporation.
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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrettyPrintingUnionType(t *testing.T) {
	t.Parallel()
	union := NewUnionType(StringType, IntType)
	pretty := union.Pretty().String()
	assert.Equal(t, "int | string", pretty)
}

func TestPrettyPrintingNestedUnionType(t *testing.T) {
	t.Parallel()
	union := NewUnionType(StringType, NewUnionType(IntType, BoolType))
	pretty := union.Pretty().String()
	assert.Equal(t, "bool | int | string", pretty)
}

func TestPrettyPrintingSelfReferencingUnionType(t *testing.T) {
	t.Parallel()
	union := &UnionType{ElementTypes: []Type{
		StringType,
		IntType,
	}}

	union = &UnionType{ElementTypes: []Type{
		StringType,
		&ListType{
			ElementType: &ObjectType{
				Properties: map[string]Type{
					"selfReferences": union,
				},
			},
		},
	}}

	pretty := union.Pretty().String()
	assert.Equal(t, "string | list({ selfReferences: string | int })", pretty)
}
