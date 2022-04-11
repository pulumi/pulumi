// Copyright 2016-2020, Pulumi Corporation.
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

package deepcopy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepCopy(t *testing.T) {
	t.Parallel()

	cases := []interface{}{
		bool(false),
		bool(true),
		int(-42),
		int8(-42),
		int16(-42),
		int32(-42),
		int64(-42),
		uint(42),
		uint8(42),
		uint16(42),
		uint32(42),
		uint64(42),
		float32(3.14159),
		float64(3.14159),
		complex64(complex(3.14159, -42)),
		complex(3.14159, -42),
		"foo",
		[2]byte{42, 24},
		[]byte{0, 1, 2, 3},
		[]string{"foo", "bar"},
		map[string]int{
			"a": 42,
			"b": 24,
		},
		struct {
			Foo int
			Bar map[int]int
		}{
			Foo: 42,
			Bar: map[int]int{
				19: 77,
			},
		},
		[]map[string]string{
			{
				"foo": "bar",
				"baz": "qux",
			},
			{
				"alpha": "beta",
			},
		},
		map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": "baz",
			},
			"bar": []int{42},
		},
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for i, c := range cases {
		i, c := i, c
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			t.Parallel()
			assert.EqualValues(t, c, Copy(c))
		})
	}
}
