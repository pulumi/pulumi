// Copyright 2017 Pulumi, Inc. All rights reserved.

package rt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/coconut/pkg/compiler/types"
)

// TestObjectRTTI tests that objects that have been created maintain correct type identities.
func TestObjectRTTI(t *testing.T) {
	// null
	nul := NewNullObject()
	assert.True(t, nul.IsNull())
	assert.Equal(t, types.Null, nul.Type())

	// true bool
	boTrue := NewBoolObject(true)
	assert.True(t, boTrue.IsBool())
	assert.Equal(t, types.Bool, boTrue.Type())
	assert.Equal(t, true, boTrue.BoolValue())
	boTrueV, boTrueOK := boTrue.TryBoolValue()
	assert.True(t, boTrueOK)
	assert.Equal(t, true, boTrueV)

	// false bool
	boFalse := NewBoolObject(true)
	assert.True(t, boFalse.IsBool())
	assert.Equal(t, types.Bool, boFalse.Type())
	assert.Equal(t, true, boFalse.BoolValue())
	boFalseV, boFalseOK := boFalse.TryBoolValue()
	assert.True(t, boFalseOK)
	assert.Equal(t, true, boFalseV)

	// 42 number
	n := float64(42)
	num := NewNumberObject(n)
	assert.True(t, num.IsNumber())
	assert.Equal(t, types.Number, num.Type())
	assert.Equal(t, n, num.NumberValue())
	numV, numOK := num.TryNumberValue()
	assert.True(t, numOK)
	assert.Equal(t, n, numV)

	// "cloudy, with a chance of rain" string
	s := "cloud, with a chance of rain"
	str := NewStringObject(s)
	assert.True(t, str.IsString())
	assert.Equal(t, types.String, str.Type())
	assert.Equal(t, s, str.StringValue())
	strV, strOK := str.TryStringValue()
	assert.True(t, strOK)
	assert.Equal(t, s, strV)

	// an array of all of the above
	elems := []*Pointer{
		NewPointer(nul, false, nil, nil),
		NewPointer(boTrue, false, nil, nil),
		NewPointer(boFalse, false, nil, nil),
		NewPointer(num, false, nil, nil),
		NewPointer(str, false, nil, nil),
	}
	arr := NewArrayObject(types.Object, &elems)
	assert.True(t, arr.IsArray())
	arrV, arrOK := arr.TryArrayValue()
	assert.True(t, arrOK)
	assert.NotNil(t, arrV)
	assert.Equal(t, 5, len(*arrV))
	for i, v := range *arrV {
		assert.Equal(t, elems[i], v)
	}
}
