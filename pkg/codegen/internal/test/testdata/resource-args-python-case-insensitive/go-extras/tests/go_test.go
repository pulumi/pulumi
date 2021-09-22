package tests

import (
	"reflect"
	"testing"

<<<<<<< HEAD:pkg/codegen/internal/test/testdata/resource-args-python/go-extras/tests/go_test.go
	"resource-args-python/example"
=======
	"resource-args-python-case-insensitive/example"
>>>>>>> master:pkg/codegen/internal/test/testdata/resource-args-python-case-insensitive/go-extras/tests/go_test.go

	"github.com/stretchr/testify/assert"
)

func TestArrayElemType(t *testing.T) {
	var arr example.PersonArray
	assert.Equal(t, reflect.TypeOf([]*example.Person(nil)), arr.ElementType())
}

func TestMapElemType(t *testing.T) {
	var m example.PersonMap
	assert.Equal(t, reflect.TypeOf(map[string]*example.Person(nil)), m.ElementType())
}
