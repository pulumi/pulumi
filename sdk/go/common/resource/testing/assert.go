package testing

import (
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func AssertEqualPropertyValues(t assert.TestingT, expected, actual resource.PropertyValue) {
	if !actual.DeepEquals(expected) {
		assert.Equal(t, expected, actual)
	}
}
