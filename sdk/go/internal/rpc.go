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
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// MapStructTypes returns a function that maps the fields of struct type 'from'
// to the fields of struct type 'to'.
//
// The returned function takes a value of type 'to'
// and an index of a field in 'from',
// and returns the corresponding field in 'to'.
// The value may be omitted if just the field's type information is needed.
func MapStructTypes(from, to reflect.Type) func(reflect.Value, int) (reflect.StructField, reflect.Value) {
	contract.Assertf(from.Kind() == reflect.Struct, "from must be a struct type, got %v (%v)", from, from.Kind())
	contract.Assertf(to.Kind() == reflect.Struct, "to must be a struct type, got %v (%v)", to, to.Kind())

	if from == to {
		return func(v reflect.Value, i int) (reflect.StructField, reflect.Value) {
			var fv reflect.Value
			if v.IsValid() {
				fv = v.Field(i)
			}
			return to.Field(i), fv
		}
	}

	nameToIndex := map[string]int{}
	numFields := to.NumField()
	for i := 0; i < numFields; i++ {
		nameToIndex[to.Field(i).Name] = i
	}

	return func(v reflect.Value, i int) (reflect.StructField, reflect.Value) {
		fieldName := from.Field(i).Name
		j, ok := nameToIndex[fieldName]
		if !ok {
			panic(fmt.Errorf("unknown field %v when marshaling inputs of type %v to %v", fieldName, from, to))
		}

		field := to.Field(j)
		var fieldValue reflect.Value
		if v.IsValid() {
			fieldValue = v.Field(j)
		}
		return field, fieldValue
	}
}
