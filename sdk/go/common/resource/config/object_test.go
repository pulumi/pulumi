package config

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestEmptyObject(t *testing.T) {
	t.Parallel()

	// Test that an empty object can be converted to a property value
	// without error.
	o := object{}
	crypter := nopCrypter{}
	v := o.toDecryptedPropertyValue(crypter)
	assert.Equal(t, resource.NewNullProperty(), v)
}
