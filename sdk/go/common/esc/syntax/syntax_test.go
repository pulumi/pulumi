// Copyright 2022, Pulumi Corporation.  All rights reserved.

package syntax

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		node     Node
		expected string
	}{
		{String("foo"), "foo"},
		{Null(), "null"},
		{Number(3.14159), "3.14159"},
		{Number(3), "3"},
		{List(String("e1"), Number(2), List(), Null()), "[ e1, 2, [ ], null ]"},
		{
			Object(ObjectProperty(String("fizz"), String("buzz")), ObjectProperty(String("empty"), Object())),
			"{ fizz: buzz, empty: { } }",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expected, c.node.String())
		})
	}
}
