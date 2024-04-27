// Copyright 2016-2024, Pulumi Corporation.
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

package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestSameIsEqual(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		same := Value(10).Draw(t, "value")
		assert.True(t, same.Equals(same))
	})
}

func TestDifferentIsNotEqual(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		arr := rapid.SliceOfNDistinct(Value(10), 2, 2, func(v property.Value) string {
			return fmt.Sprintf("%#v", v)
		}).Draw(t, "values")
		v1, v2 := arr[0], arr[1]
		assert.False(t, v1.Equals(v2))
	})
}
