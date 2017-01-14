// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type bag struct {
	Bool     bool
	BoolP    *bool
	String   string
	StringP  *string
	Float64  float64
	Float64P *float64
}

func TestDecodeField(t *testing.T) {
	tree := make(object)
	tree["b"] = true
	tree["s"] = "hello"
	tree["f"] = float64(3.14159265359)

	// Try some simple primitive decodes.
	var s bag
	var err error
	err = decodeField(tree, "bag", "b", &s.Bool, true)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], s.Bool)
	err = decodeField(tree, "bag", "b", &s.BoolP, true)
	assert.Nil(t, err)
	assert.Equal(t, tree["b"], *s.BoolP)
	err = decodeField(tree, "bag", "s", &s.String, true)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], s.String)
	err = decodeField(tree, "bag", "s", &s.StringP, true)
	assert.Nil(t, err)
	assert.Equal(t, tree["s"], *s.StringP)
	err = decodeField(tree, "bag", "f", &s.Float64, true)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], s.Float64)
	err = decodeField(tree, "bag", "f", &s.Float64P, true)
	assert.Nil(t, err)
	assert.Equal(t, tree["f"], *s.Float64P)

	// Ensure missing optional fields are ignored:
	s.String = "x"
	err = decodeField(tree, "bag", "missing", &s.String, false)
	assert.Nil(t, err)
	assert.Equal(t, "x", s.String)

	// Try some error conditions; first, wrong type:
	s.String = "x"
	err = decodeField(tree, "bag", "b", &s.String, true)
	assert.NotNil(t, err)
	assert.Equal(t, "bag `b` must be a `string`, got `bool`", err.Error())
	assert.Equal(t, "x", s.String)

	// Next, missing required field:
	s.String = "x"
	err = decodeField(tree, "bag", "missing", &s.String, true)
	assert.NotNil(t, err)
	assert.Equal(t, "Missing required bag field `missing`", err.Error())
	assert.Equal(t, "x", s.String)
}
