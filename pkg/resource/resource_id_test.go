// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"crypto/sha1"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUniqueHex(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 100
	id := NewUniqueHex(prefix, maxlen, randlen)
	assert.Equal(t, len(prefix)+randlen*2, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexMaxLen(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 20
	id := NewUniqueHex(prefix, maxlen, randlen)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueDefaults(t *testing.T) {
	prefix := "prefix"
	id := NewUniqueHex(prefix, -1, -1)
	assert.Equal(t, len(prefix)+(sha1.Size*2), len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func TestNewUniqueHexID(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 100
	id := NewUniqueHexID(prefix, maxlen, randlen)
	assert.Equal(t, len(prefix)+randlen*2, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueHexMaxLenID(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 20
	id := NewUniqueHexID(prefix, maxlen, randlen)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}

func TestNewUniqueDefaultsID(t *testing.T) {
	prefix := "prefix"
	id := NewUniqueHexID(prefix, -1, -1)
	assert.Equal(t, len(prefix)+(sha1.Size*2), len(id))
	assert.Equal(t, true, strings.HasPrefix(string(id), prefix))
}
