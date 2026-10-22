// Copyright 2026, Pulumi Corporation.
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

package pulumi

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStringAssetOutput(t *testing.T) {
	t.Parallel()

	t.Run("from plain string", func(t *testing.T) {
		t.Parallel()

		out := NewStringAssetOutput(String("hello"))
		v, known, secret, _, err := await(out)
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		require.NotNil(t, v)
		assert.Equal(t, "hello", v.(Asset).Text())
	})

	t.Run("from string output", func(t *testing.T) {
		t.Parallel()

		strOut := StringOutput{internal.NewOutputState(nil, reflect.TypeOf(""))}
		go func() {
			internal.ResolveOutput(strOut, "world", true, false, resourcesToInternal(nil))
		}()
		out := NewStringAssetOutput(strOut)
		v, known, secret, _, err := await(out)
		require.NoError(t, err)
		assert.True(t, known)
		assert.False(t, secret)
		require.NotNil(t, v)
		assert.Equal(t, "world", v.(Asset).Text())
	})

	t.Run("unknown string output produces unknown asset", func(t *testing.T) {
		t.Parallel()

		strOut := StringOutput{internal.NewOutputState(nil, reflect.TypeOf(""))}
		go func() {
			internal.ResolveOutput(strOut, "", false, false, resourcesToInternal(nil))
		}()
		out := NewStringAssetOutput(strOut)
		_, known, _, _, err := await(out)
		require.NoError(t, err)
		assert.False(t, known)
	})
}
