// Copyright 2016-2021, Pulumi Corporation.
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

func TestStringSetContains(t *testing.T) {
	set123 := NewStringSet("1", "2", "3")
	set12 := NewStringSet("1", "2")
	set14 := NewStringSet("1", "4")
	setEmpty := NewStringSet()

	assert.True(t, set123.Contains(set123))
	assert.True(t, set123.Contains(set12))
	assert.False(t, set12.Contains(set123))
	assert.False(t, set123.Contains(set14))
	assert.True(t, set123.Contains(setEmpty))
}

func TestStringSetSubtract(t *testing.T) {
	set1234 := NewStringSet("1", "2", "3", "4")
	set125 := NewStringSet("1", "2", "5")
	set34 := NewStringSet("3", "4")
	setEmpty := NewStringSet()

	assert.Equal(t, set34, set1234.Subtract(set125))
	assert.Equal(t, setEmpty, set1234.Subtract(set1234))
	assert.Equal(t, set1234, set1234.Subtract(setEmpty))
}

func TestSimplifyInputUnion(t *testing.T) {
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
