// Copyright 2022, Pulumi Corporation.  All rights reserved.

package diags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditDistance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b     string
		expected int
	}{
		{"vpcId", "cpcId", 1},
		{"vpcId", "foo", 5},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, editDistance(c.a, c.b))
	}
}
