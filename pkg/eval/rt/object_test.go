// Copyright 2016-2017, Pulumi Corporation
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

package rt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/compiler/types"
)

// TestObjectRTTI tests that objects that have been created maintain correct type identities.
func TestObjectRTTI(t *testing.T) {
	t.Parallel()

	// null
	nul := NewNullObject()
	assert.True(t, nul.IsNull())
	assert.Equal(t, types.Null, nul.Type())

	// null (predefined)
	nul2 := Null
	assert.True(t, nul2.IsNull())
	assert.Equal(t, types.Null, nul2.Type())

	// true bool
	boTrue := NewBoolObject(true)
	assert.True(t, boTrue.IsBool())
	assert.Equal(t, types.Bool, boTrue.Type())
	assert.Equal(t, true, boTrue.BoolValue())
	boTrueV, boTrueOK := boTrue.TryBoolValue()
	assert.True(t, boTrueOK)
	assert.Equal(t, true, boTrueV)

	// true bool (predefined)
	boTrue2 := True
	assert.True(t, boTrue2.IsBool())
	assert.Equal(t, types.Bool, boTrue2.Type())
	assert.Equal(t, true, boTrue2.BoolValue())
	boTrueV2, boTrueOK2 := boTrue2.TryBoolValue()
	assert.True(t, boTrueOK2)
	assert.Equal(t, true, boTrueV2)

	// false bool
	boFalse := NewBoolObject(true)
	assert.True(t, boFalse.IsBool())
	assert.Equal(t, types.Bool, boFalse.Type())
	assert.Equal(t, true, boFalse.BoolValue())
	boFalseV, boFalseOK := boFalse.TryBoolValue()
	assert.True(t, boFalseOK)
	assert.Equal(t, true, boFalseV)

	// false bool (predefined)
	boFalse2 := False
	assert.True(t, boFalse2.IsBool())
	assert.Equal(t, types.Bool, boFalse2.Type())
	assert.Equal(t, false, boFalse2.BoolValue())
	boFalseV2, boFalseOK2 := boFalse2.TryBoolValue()
	assert.True(t, boFalseOK2)
	assert.Equal(t, false, boFalseV2)

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
