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

func TestContainsObservableUnknowns(t *testing.T) {
	fake := newMissingExpr("", nil)
	cases := map[string]struct {
		value    *value
		expected bool
	}{
		"unknown is rotateOnly, so it's not observable": {
			value: &value{def: fake, repr: map[string]*value{
				"rotateOnly": {def: fake, rotateOnly: true, unknown: true},
				"plain":      {def: fake, rotateOnly: false, unknown: false, repr: "bar"},
			}},
			expected: false,
		},
		"unknown is not rotateOnly, so it's observable": {
			value: &value{def: fake, repr: map[string]*value{
				"rotateOnly": {def: fake, rotateOnly: true, unknown: false, repr: "foo"},
				"plain":      {def: fake, rotateOnly: false, unknown: true},
			}},
			expected: true,
		},
		"there are no unknowns": {
			value: &value{def: fake, repr: map[string]*value{
				"rotateOnly": {def: fake, rotateOnly: true, unknown: false, repr: "foo"},
				"plain":      {def: fake, rotateOnly: false, unknown: false, repr: "bar"},
			}},
			expected: false,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// when rotating, containsObservableUnknowns === containsUnknowns
			assert.Equal(t, c.value.containsUnknowns(), c.value.containsObservableUnknowns(true))

			// when not rotating, containsObservableUnknowns is based on rotateOnly
			assert.Equal(t, c.expected, c.value.containsObservableUnknowns(false))
		})
	}
}
