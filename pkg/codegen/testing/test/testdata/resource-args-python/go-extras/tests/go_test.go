package tests

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"resource-args-python/example"
)

func TestArrayElemType(t *testing.T) {
	var arr example.PersonArray
	assert.Equal(t, reflect.TypeOf([]*example.Person(nil)), arr.ElementType())
}

func TestMapElemType(t *testing.T) {
	var m example.PersonMap
	assert.Equal(t, reflect.TypeOf(map[string]*example.Person(nil)), m.ElementType())
}
