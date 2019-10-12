// Copyright 2016-2018, Pulumi Corporation.
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

// nolint: goconst
package resource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUniqueHex(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	randlen := 8
	maxlen := 100
	id, err := NewUniqueHex(prefix, randlen, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+randlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexMaxLen2(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	randlen := 8
	maxlen := 13
	_, err := NewUniqueHex(prefix, randlen, maxlen)
	assert.NotNil(t, err)
}

func TestNewUniqueHexEnsureRandomness2(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	// Just enough space to have 8 chars of randomenss
	randlen := 8
	maxlen := 14
	id, err := NewUniqueHex(prefix, randlen, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueDefaults(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	id, err := NewUniqueHex(prefix, -1, -1)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexID(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	randlen := 8
	maxlen := 100
	id, err := NewUniqueHexID(prefix, randlen, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueHexMaxLenID(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	randlen := 8
	maxlen := 20
	id, err := NewUniqueHexID(prefix, randlen, maxlen)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueDefaultsID(t *testing.T) {
	t.Parallel()
	prefix := "prefix"
	id, err := NewUniqueHexID(prefix, -1, -1)
	assert.Nil(t, err)
	assert.Equal(t, len(prefix)+8, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}
