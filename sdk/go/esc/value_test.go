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

package esc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSecret(t *testing.T) {
	cases := []struct {
		name      string
		newSecret func() Value
		expected  Value
	}{
		{
			name:      "bool",
			newSecret: func() Value { return NewSecret(true) },
			expected:  Value{Value: true, Secret: true},
		},
		{
			name:      "number",
			newSecret: func() Value { return NewSecret(json.Number("3.14")) },
			expected:  Value{Value: json.Number("3.14"), Secret: true},
		},
		{
			name:      "string",
			newSecret: func() Value { return NewSecret("hello") },
			expected:  Value{Value: "hello", Secret: true},
		},
		{
			name:      "array",
			newSecret: func() Value { return NewSecret([]Value{NewValue([]Value{NewValue("hello"), NewValue("world")})}) },
			expected:  Value{Value: []Value{{Value: []Value{{Value: "hello", Secret: true}, {Value: "world", Secret: true}}, Secret: true}}, Secret: true},
		},
		{
			name: "object",
			newSecret: func() Value {
				return NewSecret(map[string]Value{"nest": NewValue(map[string]Value{"hello": NewValue("world")})})
			},
			expected: Value{Value: map[string]Value{"nest": {Value: map[string]Value{"hello": {Value: "world", Secret: true}}, Secret: true}}, Secret: true},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.newSecret()
			assert.Equal(t, actual, c.expected)
		})
	}
}

func TestMakeSecret(t *testing.T) {
	cases := []struct {
		name     string
		value    Value
		expected Value
	}{
		{
			name:     "zero",
			value:    Value{},
			expected: Value{Secret: true},
		},
		{
			name:     "bool",
			value:    NewValue(true),
			expected: Value{Value: true, Secret: true},
		},
		{
			name:     "number",
			value:    NewValue(json.Number("3.14")),
			expected: Value{Value: json.Number("3.14"), Secret: true},
		},
		{
			name:     "string",
			value:    NewValue("hello"),
			expected: Value{Value: "hello", Secret: true},
		},
		{
			name:     "array",
			value:    NewValue([]Value{NewValue([]Value{NewValue("hello"), NewValue("world")})}),
			expected: Value{Value: []Value{{Value: []Value{{Value: "hello", Secret: true}, {Value: "world", Secret: true}}, Secret: true}}, Secret: true},
		},
		{
			name:     "object",
			value:    NewValue(map[string]Value{"nest": NewValue(map[string]Value{"hello": NewValue("world")})}),
			expected: Value{Value: map[string]Value{"nest": {Value: map[string]Value{"hello": {Value: "world", Secret: true}}, Secret: true}}, Secret: true},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.value.MakeSecret()
			assert.Equal(t, actual, c.expected)
		})
	}
}
