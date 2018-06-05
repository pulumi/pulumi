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
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

// tokenBuffer is a parseable token buffer that simply carries a position.
type tokenBuffer struct {
	Tok Type
	Pos int
}

func newTokenBuffer(tok Type) *tokenBuffer {
	return &tokenBuffer{
		Tok: tok,
		Pos: 0,
	}
}

func (b *tokenBuffer) Curr() Type {
	return b.Tok[b.Pos:]
}

func (b *tokenBuffer) From(from int) Type {
	return b.Tok[from:b.Pos]
}

func (b *tokenBuffer) Eat(s string) {
	ate := b.MayEat(s)
	contract.Assertf(ate, "Expected to eat '%v'", s)
}

func (b *tokenBuffer) MayEat(s string) bool {
	if strings.HasPrefix(string(b.Curr()), s) {
		b.Advance(len(s))
		return true
	}
	return false
}

func (b *tokenBuffer) Advance(by int) {
	b.Pos += by
}

func (b *tokenBuffer) Done() bool {
	return b.Pos == len(b.Tok)
}

func (b *tokenBuffer) Finish() {
	b.Pos = len(b.Tok)
}

// typePartDelims are separator characters that are used to parse recursive types.
var typePartDelims = MapTypeSeparator + FunctionTypeParamSeparator + FunctionTypeSeparator

// parseNextType parses one type out of the given token, mutating the buffer in place and returning the resulting type
// token.  This allows recursive parsing of complex decorated types below (like `map[[]string]func(func())`).
func parseNextType(b *tokenBuffer) Type {
	// First, check for decorated types.
	tok := b.Curr()
	if tok.Pointer() {
		ptr := parseNextPointerType(b)
		return ptr.Tok
	} else if tok.Array() {
		arr := parseNextArrayType(b)
		return arr.Tok
	} else if tok.Map() {
		mam := parseNextMapType(b)
		return mam.Tok
	} else if tok.Function() {
		fnc := parseNextFunctionType(b)
		return fnc.Tok
	}

	// Otherwise, we have either a qualified or simple (primitive) name.  Since we might be deep in the middle
	// of parsing another token, however, we only parse up to any other decorator termination/separator tokens.
	s := string(tok)
	sep := strings.IndexAny(s, typePartDelims)
	if sep == -1 {
		b.Finish()
		return tok
	}
	b.Advance(sep)
	return tok[:sep]
}

// PointerType is a type token that decorates an element type token turn it into a pointer: `"*" <Elem>`.
type PointerType struct {
	Tok  Type // the full pointer type token.
	Elem Type // the element portion of the pointer type token.
}

const (
	PointerTypeDecors = PointerTypePrefix + "%v"
	PointerTypePrefix = "*"
)

// NewPointerTypeName creates a new array type name from an element type.
func NewPointerTypeName(elem TypeName) TypeName {
	return TypeName(fmt.Sprintf(PointerTypeDecors, elem))
}

// NewPointerTypeToken creates a new array type token from an element type.
func NewPointerTypeToken(elem Type) Type {
	return Type(fmt.Sprintf(PointerTypeDecors, elem))
}

// IsPointerType returns true if the given type token represents an encoded pointer type.
func IsPointerType(tok Type) bool {
	return strings.HasPrefix(tok.String(), PointerTypePrefix)
}

// ParsePointerType removes the pointer decorations from a token and returns its underlying type.
func ParsePointerType(tok Type) PointerType {
	b := newTokenBuffer(tok)
	ptr := parseNextPointerType(b)
	if !b.Done() {
		contract.Failf("Did not expect anything extra after the pointer type %v; got: '%v'", tok, b.Curr())
	}
	return ptr
}

// parseNextPointerType parses the next pointer type from the given buffer.
func parseNextPointerType(b *tokenBuffer) PointerType {
	mark := b.Pos            // remember where this token begins.
	b.Eat(PointerTypePrefix) // eat the "*" part.
	elem := parseNextType(b) // parse the required element type token.
	contract.Assert(elem != "")
	return PointerType{Tok: b.From(mark), Elem: elem}
}

// ArrayType is a type token that decorates an element type token to turn it into an array: `"[]" <Elem>`.
type ArrayType struct {
	Tok  Type // the full array type token.
	Elem Type // the element portion of the array type token.
}

const (
	ArrayTypeDecors = ArrayTypePrefix + "%v"
	ArrayTypePrefix = "[]"
)

// NewArrayTypeName creates a new array type name from an element type.
func NewArrayTypeName(elem TypeName) TypeName {
	return TypeName(fmt.Sprintf(ArrayTypeDecors, elem))
}

// NewArrayTypeToken creates a new array type token from an element type.
func NewArrayTypeToken(elem Type) Type {
	return Type(fmt.Sprintf(ArrayTypeDecors, elem))
}

// IsArrayType returns true if the given type token represents an encoded pointer type.
func IsArrayType(tok Type) bool {
	return strings.HasPrefix(tok.String(), ArrayTypePrefix)
}

// ParseArrayType removes the array decorations from a token and returns its underlying type.
func ParseArrayType(tok Type) ArrayType {
	b := newTokenBuffer(tok)
	arr := parseNextArrayType(b)
	if !b.Done() {
		contract.Failf("Did not expect anything extra after the array type %v; got: '%v'", tok, b.Curr())
	}
	return arr
}

// parseNextArrayType parses the next array type from the given buffer.
func parseNextArrayType(b *tokenBuffer) ArrayType {
	mark := b.Pos            // remember where this token begins.
	b.Eat(ArrayTypePrefix)   // eat the "[]" part.
	elem := parseNextType(b) // parse the required element type token.
	contract.Assert(elem != "")
	return ArrayType{Tok: b.From(mark), Elem: elem}
}

// MapType is a type token that decorates a key and element type token to turn them into a map:
//  `"map[" <Key> "]" <Elem>`.
type MapType struct {
	Tok  Type // the full map type token.
	Key  Type // the key portion of the map type token.
	Elem Type // the element portion of the map type token.
}

const (
	MapTypeDecors    = MapTypePrefix + "%v" + MapTypeSeparator + "%v"
	MapTypePrefix    = "map["
	MapTypeSeparator = "]"
)

// NewMapTypeName creates a new map type name from an element type.
func NewMapTypeName(key TypeName, elem TypeName) TypeName {
	return TypeName(fmt.Sprintf(MapTypeDecors, key, elem))
}

// NewMapTypeToken creates a new map type token from an element type.
func NewMapTypeToken(key Type, elem Type) Type {
	return Type(fmt.Sprintf(MapTypeDecors, key, elem))
}

// IsMapType returns true if the given type token represents an encoded pointer type.
func IsMapType(tok Type) bool {
	return strings.HasPrefix(tok.String(), MapTypePrefix)
}

// ParseMapType removes the map decorations from a token and returns its underlying type.
func ParseMapType(tok Type) MapType {
	b := newTokenBuffer(tok)
	mam := parseNextMapType(b)
	if !b.Done() {
		contract.Failf("Did not expect anything extra after the map type %v; got: '%v'", tok, b.Curr())
	}
	return mam
}

// parseNextMapType parses the next map type from the given buffer.
func parseNextMapType(b *tokenBuffer) MapType {
	mark := b.Pos        // remember where this token begins.
	b.Eat(MapTypePrefix) // eat the "map[" prefix.

	// Now parse the key part.
	key := parseNextType(b)
	contract.Assert(key != "")

	// Next, we expect to find the "]" separator token; eat it.
	b.Eat(MapTypeSeparator)

	// Next, parse the element type part.
	elem := parseNextType(b)
	contract.Assert(elem != "")
	return MapType{Tok: b.From(mark), Key: key, Elem: elem}
}

// FunctionType is a type token that decorates a set of optional parameter and return tokens to turn them into a
// function type: `(" [ <Param1> [ "," <ParamN> ]* ] ")" [ <Return> ]`).
type FunctionType struct {
	Tok        Type   // the full map type token.
	Parameters []Type // the parameter parts of the type token.
	Return     *Type  // the (optional) return part of the type token.
}

const (
	FunctionTypeDecors         = FunctionTypePrefix + "%v" + FunctionTypeSeparator + "%v"
	FunctionTypePrefix         = "("
	FunctionTypeParamSeparator = ","
	FunctionTypeSeparator      = ")"
)

// NewFunctionTypeName creates a new function type token from parameter and return types.
func NewFunctionTypeName(params []TypeName, ret *TypeName) TypeName {
	// Stringify the parameters (if any).
	sparams := ""
	for i, param := range params {
		if i > 0 {
			sparams += FunctionTypeParamSeparator
		}
		sparams += string(param)
	}

	// Stringify the return type (if any).
	sret := ""
	if ret != nil {
		sret = string(*ret)
	}

	return TypeName(fmt.Sprintf(FunctionTypeDecors, sparams, sret))
}

// NewFunctionTypeToken creates a new function type token from parameter and return types.
func NewFunctionTypeToken(params []Type, ret *Type) Type {
	// Stringify the parameters (if any).
	sparams := ""
	for i, param := range params {
		if i > 0 {
			sparams += FunctionTypeParamSeparator
		}
		sparams += string(param)
	}

	// Stringify the return type (if any).
	sret := ""
	if ret != nil {
		sret = string(*ret)
	}

	return Type(fmt.Sprintf(FunctionTypeDecors, sparams, sret))
}

// IsFunctionType returns true if the given type token represents an encoded pointer type.
func IsFunctionType(tok Type) bool {
	return strings.HasPrefix(tok.String(), FunctionTypePrefix)
}

// ParseFunctionType removes the function decorations from a token and returns its underlying type.
func ParseFunctionType(tok Type) FunctionType {
	b := newTokenBuffer(tok)
	fnc := parseNextFunctionType(b)
	if !b.Done() {
		contract.Failf("Did not expect anything extra after the function type %v; got: '%v'", tok, b.Curr())
	}
	return fnc
}

// parseNextFunctionType parses the next function type from the given token, returning any excess.
func parseNextFunctionType(b *tokenBuffer) FunctionType {
	mark := b.Pos             // remember the start of this token.
	b.Eat(FunctionTypePrefix) // eat the function prefix "(".

	// Parse out parameters until we encounter and eat a ")".
	var params []Type
	for !b.MayEat(FunctionTypeSeparator) {
		next := parseNextType(b)
		if next == "" {
			contract.Assert(strings.HasPrefix(string(b.Curr()), FunctionTypeSeparator))
		} else {
			params = append(params, next)

			// Eat the separator, if any, and keep going.
			if !b.MayEat(FunctionTypeParamSeparator) {
				contract.Assert(strings.HasPrefix(string(b.Curr()), FunctionTypeSeparator))
			}
		}
	}

	// Next, if there is anything remaining, parse out the return type.
	var ret *Type
	if !b.Done() {
		if rett := parseNextType(b); rett != "" {
			ret = &rett
		}
	}

	return FunctionType{Tok: b.From(mark), Parameters: params, Return: ret}
}
