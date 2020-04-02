package gen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputUsage(t *testing.T) {
	arrayUsage := getInputUsage("FooArray")
	assert.Equal(t, "Construct a concrete instance of FooArrayInput via:\n\tFooArray{ FooArgs{...} }", arrayUsage)

	mapUsage := getInputUsage("FooMap")
	assert.Equal(t, "Construct a concrete instance of FooMapInput via:\n\tFooMap{ \"key\": FooArgs{...} }", mapUsage)

	ptrUsage := getInputUsage("FooPtr")
	assert.Equal(t, "Construct a concrete instance of FooPtrInput via:\n\tFooArgs{...}.ToFooPtrOutput()", ptrUsage)

	usage := getInputUsage("Foo")
	assert.Equal(t, "Construct a concrete instance of FooInput via:\n\tFooArgs{...}", usage)
}
