// Copyright 2026, Pulumi Corporation.
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

package codegen

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestSimplifyInputUnion(t *testing.T) {
	t.Parallel()

	u1 := &schema.UnionType{
		ElementTypes: []schema.Type{
			&schema.InputType{ElementType: schema.StringType},
			schema.NumberType,
		},
	}

	u2 := SimplifyInputUnion(u1)
	assert.Equal(t, &schema.UnionType{
		ElementTypes: []schema.Type{
			schema.StringType,
			schema.NumberType,
		},
	}, u2)
}

func TestPushOptionalIntoInput(t *testing.T) {
	t.Parallel()

	// Optional(Input(T)) → Input(Optional(T))
	t.Run("optional input", func(t *testing.T) {
		t.Parallel()
		typ := &schema.OptionalType{
			ElementType: &schema.InputType{ElementType: schema.StringType},
		}
		result := PushOptionalIntoInput(typ)
		assert.Equal(t, &schema.InputType{
			ElementType: &schema.OptionalType{ElementType: schema.StringType},
		}, result)
	})

	// Also simplifies inner union: Optional(Input(Union(Input(A), Input(B)))) → Input(Optional(Union(A, B)))
	t.Run("optional input with union", func(t *testing.T) {
		t.Parallel()
		typ := &schema.OptionalType{
			ElementType: &schema.InputType{
				ElementType: &schema.UnionType{
					ElementTypes: []schema.Type{
						&schema.InputType{ElementType: schema.StringType},
						&schema.InputType{ElementType: schema.NumberType},
					},
				},
			},
		}
		result := PushOptionalIntoInput(typ)
		assert.Equal(t, &schema.InputType{
			ElementType: &schema.OptionalType{
				ElementType: &schema.UnionType{
					ElementTypes: []schema.Type{
						schema.StringType,
						schema.NumberType,
					},
				},
			},
		}, result)
	})

	// Non-optional types are returned unchanged
	t.Run("input without optional", func(t *testing.T) {
		t.Parallel()
		typ := &schema.InputType{ElementType: schema.StringType}
		assert.Equal(t, typ, PushOptionalIntoInput(typ))
	})

	// Optional without Input is returned unchanged
	t.Run("optional without input", func(t *testing.T) {
		t.Parallel()
		typ := &schema.OptionalType{ElementType: schema.StringType}
		assert.Equal(t, typ, PushOptionalIntoInput(typ))
	})

	// Plain types are returned unchanged
	t.Run("plain type", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, schema.StringType, PushOptionalIntoInput(schema.StringType))
	})
}
