package resource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewUniqueHex(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 100
	id := NewUniqueHex(prefix, maxlen, randlen)
	assert.Equal(t, len(prefix)+randlen*2, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func Test_NewUniqueHex_Maxlen(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 20
	id := NewUniqueHex(prefix, maxlen, randlen)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}
