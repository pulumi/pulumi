package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpaqueEquality(t *testing.T) {
	t.Parallel()

	x := NewOpaqueType("x")
	x2 := NewOpaqueType("x")

	assert.True(t, x.Equals(x2))
	assert.True(t, x2.Equals(x))

	assert.True(t, x.equals(x2, map[Type]struct{}{}))
	assert.True(t, x2.equals(x, map[Type]struct{}{}))
}

func TestOpaqueInequality(t *testing.T) {
	t.Parallel()

	x := NewOpaqueType("x")
	y := NewOpaqueType("y")

	assert.False(t, x.Equals(y))
	assert.False(t, y.Equals(x))

	assert.False(t, x.equals(y, map[Type]struct{}{}))
	assert.False(t, y.equals(x, map[Type]struct{}{}))
}
