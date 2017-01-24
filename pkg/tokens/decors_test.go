// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestTypeToken(nm string) Type {
	pkg := NewPackageToken("test/package")
	mod := NewModuleToken(pkg, "test/module")
	return NewTypeToken(mod, TypeName(nm))
}

func TestArrayTypes(t *testing.T) {
	// Test simple primitives.
	for _, prim := range []string{"any", "bool", "string", "number"} {
		ptr := NewArrayTypeToken(Type(prim))
		assert.True(t, ptr.Array(), "Expected array type token to be an array")
		parsed := ParseArrayType(ptr)
		assert.Equal(t, prim, string(parsed.Elem))
	}

	// Test more complex array type elements.
	class := newTestTypeToken("ArrayTest")
	ptr := NewArrayTypeToken(class)
	assert.True(t, ptr.Array(), "Expected array type token to be an array")
	parsed := ParseArrayType(ptr)
	assert.Equal(t, string(class), string(parsed.Elem))
}

func TestPointerTypes(t *testing.T) {
	// Test simple primitives.
	for _, prim := range []string{"any", "bool", "string", "number"} {
		ptr := NewPointerTypeToken(Type(prim))
		assert.True(t, ptr.Pointer(), "Expected pointer type token to be a pointer")
		parsed := ParsePointerType(ptr)
		assert.Equal(t, prim, string(parsed.Elem))
	}

	// Test more complex pointer type elements.
	class := newTestTypeToken("PointerTest")
	ptr := NewPointerTypeToken(class)
	assert.True(t, ptr.Pointer(), "Expected pointer type token to be a pointer")
	parsed := ParsePointerType(ptr)
	assert.Equal(t, string(class), string(parsed.Elem))
}

func TestMapTypes(t *testing.T) {
	// Test simple primitives.
	for _, key := range []string{"string", "bool", "number"} {
		for _, elem := range []string{"any", "bool", "string", "number"} {
			ptr := NewMapTypeToken(Type(key), Type(elem))
			assert.True(t, ptr.Map(), "Expected map type token to be a map")
			parsed := ParseMapType(ptr)
			assert.Equal(t, key, string(parsed.Key))
			assert.Equal(t, elem, string(parsed.Elem))
		}
	}

	// Test more complex map type elements.
	for _, key := range []string{"string", "bool", "number"} {
		class := newTestTypeToken("MapTest")
		ptr := NewMapTypeToken(Type(key), class)
		assert.True(t, ptr.Map(), "Expected map type token to be a map")
		parsed := ParseMapType(ptr)
		assert.Equal(t, key, string(parsed.Key))
		assert.Equal(t, string(class), string(parsed.Elem))
	}
}

func TestFunctionTypes(t *testing.T) {
	class := newTestTypeToken("FuncTest")
	types := []string{"any", "bool", "string", "number", string(class)}
	rtypes := append([]string{""}, types...)
	for _, retty := range rtypes {
		// If the return exists, use it.
		var ret *Type
		if retty != "" {
			r := Type(retty)
			ret = &r
		}

		// Do [0...4) parameter counts.
		for i := 0; i < 4; i++ {
			ixs := make([]int, i)
			for {
				// Append the current set to the params.
				var params []Type
				for _, ix := range ixs {
					params = append(params, Type(types[ix]))
				}

				// Check the result.
				fnc := NewFunctionTypeToken(params, ret)
				assert.True(t, fnc.Function(), "Expected function type token to be a function")
				parsed := ParseFunctionType(fnc)
				assert.Equal(t, len(params), len(parsed.Parameters))
				for i, param := range parsed.Parameters {
					assert.Equal(t, string(params[i]), string(param))
				}
				if ret == nil {
					assert.Nil(t, parsed.Return)
				} else {
					assert.NotNil(t, parsed.Return)
					assert.Equal(t, string(*ret), string(*parsed.Return))
				}

				// Now rotate the parameters (or break if done).
				done := (i == 0)
				for j := 0; j < i; j++ {
					ixs[j]++
					if ixs[j] == len(types) {
						// Reset the counter, and keep incrementing.
						ixs[j] = 0
						if j == i-1 {
							// Done altogether; break break break!
							done = true
						}
					} else {
						// The lower indices aren't exhausted, stop incrementing.
						break
					}
				}
				if done {
					break
				}
			}
		}
	}
}
