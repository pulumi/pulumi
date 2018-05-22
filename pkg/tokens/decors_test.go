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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestComplexTypes(t *testing.T) {
	t.Parallel()

	// Create a crazy nested type and make sure they parse correctly; essentially:
	//		*[]map[string]map[()*(bool,string,Crazy)number][][]Crazy
	// or, in the fully qualified form:
	//		*[]map[string]map[()*(bool,string,test/package:test/module/Crazy)number][][]test/package:test/module/Crazy
	// which should parse as
	//		Pointer
	//			Elem=Array
	//				Elem=Map
	//					Key=string
	//					Elem=Map
	//						Key=Func
	//							Params=()
	//							Return=Pointer
	//								Func
	//									Params=
	//										bool
	//										string
	//										Crazy
	//									Return=number
	//						Elem=Array
	//							Elem=Array
	//								Elem=Crazy
	crazy := newTestTypeToken("Crazy")

	number := Type("number")
	ptrret := NewPointerTypeToken(NewFunctionTypeToken([]Type{"bool", "string", crazy}, &number))

	ptr := NewPointerTypeToken(
		NewArrayTypeToken(
			NewMapTypeToken(
				Type("string"),
				NewMapTypeToken(
					NewFunctionTypeToken(
						[]Type{},
						&ptrret,
					),
					NewArrayTypeToken(
						NewArrayTypeToken(
							crazy,
						),
					),
				),
			),
		),
	)

	assert.True(t, ptr.Pointer(), "Expected pointer type token to be an pointer")
	p1 := ParsePointerType(ptr) // Pointer<Array>
	{
		assert.True(t, p1.Elem.Array())
		p2 := ParseArrayType(p1.Elem) // Array<Map>
		{
			assert.True(t, p2.Elem.Map())
			p3 := ParseMapType(p2.Elem) // Map<string, Map>
			{
				assert.Equal(t, "string", string(p3.Key))
				assert.True(t, p3.Elem.Map())
				p4 := ParseMapType(p3.Elem) // Map<Func, Array>
				{
					assert.True(t, p4.Key.Function())
					p5 := ParseFunctionType(p4.Key) // Func<(), Pointer>
					{
						assert.Equal(t, 0, len(p5.Parameters))
						assert.NotNil(t, p5.Return)
						assert.True(t, (*p5.Return).Pointer())
						p6 := ParsePointerType(*p5.Return) // Pointer<Func>
						{
							assert.True(t, p6.Elem.Function())
							p7 := ParseFunctionType(p6.Elem) // Func<(bool,string,Crazy), number>
							{
								assert.Equal(t, 3, len(p7.Parameters))
								assert.Equal(t, "bool", string(p7.Parameters[0]))
								assert.Equal(t, "string", string(p7.Parameters[1]))
								assert.Equal(t, string(crazy), string(p7.Parameters[2]))
								assert.NotNil(t, p7.Return)
								assert.Equal(t, "number", string(*p7.Return))
							}
						}
					}
					assert.True(t, p4.Elem.Array())
					p8 := ParseArrayType(p4.Elem) // Array<Array>
					{
						assert.True(t, p8.Elem.Array())
						p9 := ParseArrayType(p8.Elem) // Array<Crazy>
						{
							assert.Equal(t, string(crazy), string(p9.Elem))
						}
					}
				}
			}
		}
	}
}
