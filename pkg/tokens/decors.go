// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuIL symbol and token types.
package tokens

import (
	"fmt"
	"strings"

	"github.com/marapongo/mu/pkg/util/contract"
)

// typePartDelims are separator characters that are used to parse recursive types.
var typePartDelims = MapTypeSeparator + FunctionTypeParamSeparator + FunctionTypeSeparator

// parseNextType parses one type out of the given token, returning both the resulting type token plus the remainder of
// the string.  This allows recursive parsing of complex decorated types below (like `map[[]string]func(func())`).
func parseNextType(tok Type) (Type, string) {
	// First, check for decorated types.
	if tok.Pointer() {
		ptr, rest := parseNextPointerType(tok)
		return ptr.Tok, rest
	} else if tok.Array() {
		arr, rest := parseNextArrayType(tok)
		return arr.Tok, rest
	} else if tok.Map() {
		mam, rest := parseNextMapType(tok)
		return mam.Tok, rest
	} else if tok.Function() {
		fnc, rest := parseNextFunctionType(tok)
		return fnc.Tok, rest
	} else {
		// Otherwise, we have either a qualified or simple (primitive) name.  Since we might be deep in the middle
		// of parsing another token, however, we only parse up to any other decorator termination/separator tokens.
		s := string(tok)
		sep := strings.IndexAny(s, typePartDelims)
		if sep == -1 {
			return tok, ""
		} else {
			return tok[:sep], s[sep:]
		}
	}
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
	ptr, extra := parseNextPointerType(tok)
	contract.Assertf(extra == "", "Did not expect anything extra after the pointer type %v; got: '%v'", tok, extra)
	return ptr
}

// parseNextPointerType parses the next pointer type from the given token, returning any excess.
func parseNextPointerType(tok Type) (PointerType, string) {
	contract.Requiref(IsPointerType(tok), "tok", "IsPointerType")
	rest := string(tok)[len(PointerTypePrefix):]
	elem, rest := parseNextType(Type(rest))
	return PointerType{tok, elem}, rest
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
	ptr, extra := parseNextArrayType(tok)
	contract.Assertf(extra == "", "Did not expect anything extra after the array type %v; got: '%v'", tok, extra)
	return ptr
}

// parseNextArrayType parses the next array type from the given token, returning any excess.
func parseNextArrayType(tok Type) (ArrayType, string) {
	contract.Requiref(IsArrayType(tok), "tok", "IsArrayType")
	rest := string(tok)[len(ArrayTypePrefix):]
	elem, rest := parseNextType(Type(rest))
	return ArrayType{tok, elem}, rest
}

// MapType is a type token that decorates a key and element type token to turn them into a map: `"map[" <Key> "]" <Elem>`.
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
	ptr, extra := parseNextMapType(tok)
	contract.Assertf(extra == "", "Did not expect anything extra after the map type %v; got: '%v'", tok, extra)
	return ptr
}

// parseNextMapType parses the next map type from the given token, returning any excess.
func parseNextMapType(tok Type) (MapType, string) {
	contract.Requiref(IsMapType(tok), "tok", "IsMapType")

	// Strip off the "map[" part.
	rest := string(tok)[len(MapTypePrefix):]

	// Now parse the key part.
	key, rest := parseNextType(Type(rest))

	// Next, we expect to find the "]" separator token; eat it.
	contract.Assertf(len(rest) > 0 && strings.HasPrefix(rest, MapTypeSeparator), "Expected a map separator")
	rest = rest[1:]

	// Next, parse the element type part.
	elem, rest := parseNextType(Type(rest))
	return MapType{tok, key, elem}, rest
}

// FunctionType is a type token that decorates a set of optional parameter and return tokens to turn them into a function
// type: `(" [ <Param1> [ "," <ParamN> ]* ] ")" [ <Return> ]`).
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
	ptr, extra := parseNextFunctionType(tok)
	contract.Assertf(extra == "", "Did not expect anything extra after the function type %v; got: '%v'", tok, extra)
	return ptr
}

// parseNextFunctionType parses the next function type from the given token, returning any excess.
func parseNextFunctionType(tok Type) (FunctionType, string) {
	contract.Requiref(IsFunctionType(tok), "tok", "IsFunctionType")

	// Strip off the "(" part.
	rest := string(tok)[len(FunctionTypePrefix):]

	var params []Type
	for {
		if comma := strings.Index(rest, FunctionTypeParamSeparator); comma != -1 {
			// More parameters are coming... parse up to the comma.
			params = append(params, Type(rest[:comma]))
			rest = rest[comma+1:]
			contract.Assert(len(rest) > 0 && !strings.HasPrefix(rest, FunctionTypeSeparator))
		} else {
			// The end is in sight.  Maybe there's more, maybe not.
			if term := strings.Index(rest, FunctionTypeSeparator); term != 0 {
				params = append(params, Type(rest[:term]))
			}
			break
		}
	}

	// Next, we expect to find the ")" separator token.
	contract.Assertf(
		len(rest) > 0 && strings.HasPrefix(rest, FunctionTypeSeparator), "Expected a function separator")

	// Next, if there is anything remaining, parse out the return type.
	var ret *Type
	if rest != "" {
		var rett Type
		rett, rest = parseNextType(Type(rest))
		ret = &rett
	}

	return FunctionType{tok, params, ret}, rest
}
