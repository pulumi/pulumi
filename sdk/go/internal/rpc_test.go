// Copyright 2016-2023, Pulumi Corporation.
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

package internal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapStructTypes(t *testing.T) {
	t.Parallel()

	type A struct {
		Foo string
		Bar int
	}

	type B struct {
		Bar int
		Foo string
	}

	atype := reflect.TypeOf(A{})
	btype := reflect.TypeOf(B{})
	getMappedField := MapStructTypes(atype, btype)

	// We'll build two structs with the same fields
	// but in different orders.
	//
	// Given similar values, we should be able to
	// map the fields and compare the two structs.

	a := reflect.ValueOf(A{Foo: "foo", Bar: 42})
	b := reflect.ValueOf(B{Foo: "foo", Bar: 42})
	for i := 0; i < atype.NumField(); i++ {
		aval := a.Field(i)
		bfield, bval := getMappedField(b, i)
		assert.Equal(t, aval.Interface(), bval.Interface(),
			"a.%v != b.%v", atype.Field(i).Name, bfield.Name)
	}
}

func TestMapStructTypes_sameType(t *testing.T) {
	t.Parallel()

	type A struct {
		Foo string
		Bar int
	}

	atype := reflect.TypeOf(A{})
	getMappedField := MapStructTypes(atype, atype)

	// Values returned by getMappedField
	// should match the same indexes in the original struct.
	a := reflect.ValueOf(A{Foo: "foo", Bar: 42})
	for i := 0; i < atype.NumField(); i++ {
		wantField := atype.Field(i)
		wantValue := a.Field(i)

		gotField, gotValue := getMappedField(a, i)
		assert.Equal(t, wantField.Name, gotField.Name)
		assert.Equal(t, wantValue.Interface(), gotValue.Interface())
	}
}
