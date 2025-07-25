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

package resource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUniqueHex(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	randlen := 8
	maxlen := 100
	id, err := NewUniqueHex(prefix, randlen, maxlen)
	require.NoError(t, err)
	assert.Equal(t, len(prefix)+randlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexNonDeterminism(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	randlen := 8
	maxlen := 100
	id1, err := NewUniqueHex(prefix, randlen, maxlen)
	require.NoError(t, err)
	id2, err := NewUniqueHex(prefix, randlen, maxlen)
	require.NoError(t, err)
	assert.NotEqual(t, id1, id2)
}

func TestNewUniqueHexMaxLen2(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	randlen := 8
	maxlen := 13
	_, err := NewUniqueHex(prefix, randlen, maxlen)
	assert.ErrorContains(t, err, "name 'prefix' plus 8 random chars is longer than maximum length 13")
}

func TestNewUniqueHexEnsureRandomness2(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	// Just enough space to have 8 chars of randomenss
	randlen := 8
	maxlen := 14
	id, err := NewUniqueHex(prefix, randlen, maxlen)
	require.NoError(t, err)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueDefaults(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	id, err := NewUniqueHex(prefix, -1, -1)
	require.NoError(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexID(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	randlen := 8
	maxlen := 100
	id, err := NewUniqueHexID(prefix, randlen, maxlen)
	require.NoError(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueHexMaxLenID(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	randlen := 8
	maxlen := 20
	id, err := NewUniqueHexID(prefix, randlen, maxlen)
	require.NoError(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueDefaultsID(t *testing.T) {
	t.Parallel()

	prefix := "prefix"
	id, err := NewUniqueHexID(prefix, -1, -1)
	require.NoError(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestUniqueNameDeterminism(t *testing.T) {
	t.Parallel()

	randomSeed := []byte{0, 1, 2, 3, 4}
	prefix := "prefix"
	randlen := 7
	maxlen := 100
	randchars := []rune("xyzw")
	name, err := NewUniqueName(randomSeed, prefix, randlen, maxlen, randchars)
	require.NoError(t, err)
	assert.Equal(t, "prefixywzwwyz", name)
}

func TestUniqueNameNonDeterminism(t *testing.T) {
	t.Parallel()

	// if randomSeed is nil or empty we should be nondeterministic
	for _, randomSeed := range [][]byte{nil, make([]byte, 0)} {
		prefix := "prefix"
		randlen := 4
		maxlen := 100
		name, err := NewUniqueName(randomSeed, prefix, randlen, maxlen, nil)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(name, prefix), "%s does not have prefix %s", name, prefix)
		require.Len(t, name, len(prefix)+randlen)

		name2, err := NewUniqueName(randomSeed, prefix, randlen, maxlen, nil)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(name2, prefix), "%s does not have prefix %s", name2, prefix)
		require.Len(t, name2, len(prefix)+randlen)
		assert.NotEqual(t, name, name2)
	}
}
