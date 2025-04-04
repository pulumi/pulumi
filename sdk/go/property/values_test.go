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

package property

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
)

// Calling == does not implement desirable behavior, so we ensure that it is invalid.
func TestCannotCompareValues(t *testing.T) {
	t.Parallel()
	assert.False(t, reflect.TypeOf(Value{}).Comparable())
}

func TestNullEquivalence(t *testing.T) {
	t.Parallel()
	assert.Nil(t, New(Null).v)

	assert.True(t, New(Null).Equals(Value{}))

	assert.True(t, New(Null).IsNull())
}

// This test locks in the nil vs null behavior.
//
// In go, nil is a value but not a type. For example:
//
//	(any)([]A(nil)) != nil
//
// This is because when comparing values typed as `any` still carry their underlying type
// information. This is true even when:
//
//	[]A(nil) == nil
//
// Unlike the Go language, Value's Null is not typed. This means that:
//
//	New[Array](nil).Equals(New(Null))
//
// Further, since Null is it's own type, it is the case that:
//
//	New[Array](nil).IsArray() == false
//
//	New[Map](nil).IsNull() == true
func TestNil(t *testing.T) {
	t.Parallel()

	var nullValue Value

	// []T based type, zero value is Array(nil)
	t.Run("array", func(t *testing.T) {
		t.Parallel()
		nilArray := New[[]Value](nil)

		assert.False(t, nilArray.IsArray())
		assert.True(t, nilArray.IsNull())
		assert.True(t, nilArray.Equals(nullValue))

		assert.True(t, New(Array{}).IsArray())
	})

	t.Run("map", func(t *testing.T) {
		t.Parallel()
		nilMap := New[map[string]Value](nil)

		assert.False(t, nilMap.IsMap())
		assert.True(t, nilMap.IsNull())
		assert.True(t, nilMap.Equals(nullValue))

		assert.True(t, New(Map{}).IsMap())
	})

	// *T based type, zero value is *resource.Asset(nil)
	t.Run("asset", func(t *testing.T) {
		t.Parallel()
		nilAsset := New[Asset](nil)

		assert.False(t, nilAsset.IsAsset())
		assert.True(t, nilAsset.IsNull())
		assert.True(t, nilAsset.Equals(nullValue))

		a, err := asset.FromText("")
		require.NoError(t, err)
		assert.True(t, New(a).IsAsset())
	})

	// string based type, zero value is ""
	t.Run("string", func(t *testing.T) {
		t.Parallel()
		emptyString := New[string]("")

		assert.True(t, emptyString.IsString())
		assert.False(t, emptyString.IsNull())
		assert.False(t, emptyString.Equals(nullValue))
	})
}
