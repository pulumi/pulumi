// Copyright 2023, Pulumi Corporation.

package eval

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueToString(t *testing.T) {
	cases := []struct {
		value    *value
		unknown  bool
		secret   bool
		expected string
	}{
		{
			value:    &value{unknown: true},
			unknown:  true,
			expected: "[unknown]",
		},
		{
			value:    &value{repr: false},
			expected: "false",
		},
		{
			value:    &value{repr: true},
			expected: "true",
		},
		{
			value:    &value{repr: json.Number("42")},
			expected: "42",
		},
		{
			value:    &value{repr: "foo"},
			expected: "foo",
		},
		{
			value:    &value{repr: []*value{{repr: "hello"}, {repr: "world"}}},
			expected: `"hello","world"`,
		},
		{
			value:    &value{repr: []*value{{unknown: true}, {repr: "world"}}},
			unknown:  true,
			expected: `"[unknown]","world"`,
		},
		{
			value:    &value{repr: []*value{{repr: "hello"}, {repr: "world", secret: true}}},
			secret:   true,
			expected: `"hello","world"`,
		},
		{
			value:    &value{repr: map[string]*value{"foo": {repr: "bar"}, "baz": {repr: json.Number("42")}}},
			expected: `"baz"="42","foo"="bar"`,
		},
		{
			value:    &value{repr: map[string]*value{"foo": {unknown: true}, "baz": {repr: json.Number("42")}}},
			unknown:  true,
			expected: `"baz"="42","foo"="[unknown]"`,
		},
		{
			value:    &value{repr: map[string]*value{"foo": {repr: "bar", secret: true}, "baz": {repr: json.Number("42")}}},
			secret:   true,
			expected: `"baz"="42","foo"="bar"`,
		},
	}
	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			str, unknown, secret := c.value.toString()
			assert.Equal(t, c.expected, str)
			assert.Equal(t, c.unknown, unknown)
			assert.Equal(t, c.secret, secret)
		})
	}
}
