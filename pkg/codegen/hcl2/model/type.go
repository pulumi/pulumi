// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
	"github.com/zclconf/go-cty/cty"
)

// Type represents a datatype in the Pulumi Schema. Types created by this package are identical if they are
// equal values.
type Type interface {
	Definition

	AssignableFrom(src Type) bool
	String() string

	isType()
}

// None represents the set of undefined values.
var None Type = nil

type primitiveType int

const (
	boolType    primitiveType = 1
	intType     primitiveType = 2
	numberType  primitiveType = 3
	stringType  primitiveType = 4
	archiveType primitiveType = 5
	assetType   primitiveType = 6
	anyType     primitiveType = 7
)

func (t primitiveType) AssignableFrom(src Type) bool {
	if t == anyType || src == anyType {
		return true
	}

	return src == t
}

func (t primitiveType) String() string {
	switch t {
	case boolType:
		return "boolean"
	case intType:
		return "integer"
	case numberType:
		return "number"
	case stringType:
		return "string"
	case archiveType:
		return "pulumi:pulumi:Archive"
	case assetType:
		return "pulumi:pulumi:Asset"
	case anyType:
		return "pulumi:pulumi:Any"
	default:
		panic("unknown primitive type")
	}
}

func (primitiveType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t primitiveType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	if t == anyType {
		return anyType, nil
	}
	return anyType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
}

func (primitiveType) isType() {}

// IsPrimitiveType returns true if the given Type is a primitive type. The primitive types are bool, int, number,
// string, archive, asset, and any.
func IsPrimitiveType(t Type) bool {
	_, ok := t.(primitiveType)
	return ok
}

var (
	// BoolType represents the set of boolean values.
	BoolType Type = boolType
	// IntType represents the set of 32-bit integer values.
	IntType Type = intType
	// NumberType represents the set of IEEE754 double-precision values.
	NumberType Type = numberType
	// StringType represents the set of UTF-8 string values.
	StringType Type = stringType
	// ArchiveType represents the set of Pulumi Archive values.
	ArchiveType Type = archiveType
	// AssetType represents the set of Pulumi Asset values.
	AssetType Type = assetType
	// AnyType represents the complete set of values.
	AnyType Type = anyType
)

// OptionalType represents values of a particular type that are optional.
//
// Note: we could construct this out of an undefined type and a union type, but that seems awfully fancy for our
// purposes.
type OptionalType struct {
	// ElementType is the non-optional element type.
	ElementType Type

	s string
}

// The set of optional types, indexed by element type.
var optionalTypes = map[Type]*OptionalType{}

// NewOptionalType creates a new optional type with the given element type. If the element type is itself an optional
// type, the result is the element type.
func NewOptionalType(elementType Type) *OptionalType {
	if t, ok := elementType.(*OptionalType); ok {
		return t
	}
	if t, ok := optionalTypes[elementType]; ok {
		return t
	}

	t := &OptionalType{ElementType: elementType}
	optionalTypes[elementType] = t
	return t
}

func (*OptionalType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *OptionalType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	element, diagnostics := t.ElementType.Traverse(traverser)
	return NewOptionalType(element.(Type)), diagnostics
}

// AssignableFrom returns true if this type is assignable from the indicated source type. An optional(T) is assignable
// from values of type any, none, optional(U), and U, where T is assignable from U.
func (t *OptionalType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t || src == None {
		return true
	}

	if other, ok := src.(*OptionalType); ok {
		return t.ElementType.AssignableFrom(other.ElementType)
	}

	return t.ElementType.AssignableFrom(src)
}

func (t *OptionalType) String() string {
	if t.s == "" {
		t.s = fmt.Sprintf("optional(%v)", t.ElementType)
	}
	return t.s
}

func (t *OptionalType) isType() {}

// OutputType represents eventual values that carry dependency information (e.g. resource output properties)
type OutputType struct {
	// ElementType is the element type of the output.
	ElementType Type

	s string
}

// The set of output types, indexed by element type.
var outputTypes = map[Type]*OutputType{}

// NewOutputType creates a new output type with the given element type after replacing any output or promise types
// within the element type with their respective element types.
func NewOutputType(elementType Type) *OutputType {
	elementType = ResolveOutputs(elementType)
	if t, ok := outputTypes[elementType]; ok {
		return t
	}

	t := &OutputType{ElementType: elementType}
	outputTypes[elementType] = t
	return t
}

func (*OutputType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *OutputType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	element, diagnostics := t.ElementType.Traverse(traverser)
	return NewOutputType(element.(Type)), diagnostics
}

// AssignableFrom returns true if this type is assignable from the indicated source type. An output(T) is assignable
// from values of type any, output(U), promise(U), and U, where T is assignable from U.
func (t *OutputType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t {
		return true
	}

	switch src := src.(type) {
	case *OutputType:
		return t.ElementType.AssignableFrom(src.ElementType)
	case *PromiseType:
		return t.ElementType.AssignableFrom(src.ElementType)
	}
	return t.ElementType.AssignableFrom(src)
}

func (t *OutputType) String() string {
	if t.s == "" {
		t.s = fmt.Sprintf("output(%v)", t.ElementType)
	}
	return t.s
}

func (t *OutputType) isType() {}

type typeTransform int

var (
	makeIdentity = typeTransform(0)
	makePromise  = typeTransform(1)
	makeOutput   = typeTransform(2)
)

func (f typeTransform) do(t Type) Type {
	switch f {
	case makePromise:
		return NewPromiseType(t)
	case makeOutput:
		return NewOutputType(t)
	default:
		return t
	}
}

func resolveEventuals(t Type, resolveOutputs bool) (Type, typeTransform) {
	switch t := t.(type) {
	case *OptionalType:
		resolved, transform := resolveEventuals(t.ElementType, resolveOutputs)
		return NewOptionalType(resolved), transform
	case *OutputType:
		if resolveOutputs {
			return t.ElementType, makeOutput
		}
		return t, makeIdentity
	case *PromiseType:
		return t.ElementType, makePromise
	case *MapType:
		resolved, transform := resolveEventuals(t.ElementType, resolveOutputs)
		return NewMapType(resolved), transform
	case *ArrayType:
		resolved, transform := resolveEventuals(t.ElementType, resolveOutputs)
		return NewArrayType(resolved), transform
	case *UnionType:
		transform := makeIdentity
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			element, elementTransform := resolveEventuals(t, resolveOutputs)
			if transform == makeIdentity || transform != makeOutput {
				transform = elementTransform
			}
			elementTypes[i] = element
		}
		return NewUnionType(elementTypes[0], elementTypes[1], elementTypes[2:]...), transform
	case *ObjectType:
		transform := makeIdentity
		properties := map[string]Type{}
		for k, t := range t.Properties {
			property, propertyTransform := resolveEventuals(t, resolveOutputs)
			if transform == makeIdentity || transform != makeOutput {
				transform = propertyTransform
			}
			properties[k] = property
		}
		return NewObjectType(properties), transform
	default:
		return t, makeIdentity
	}
}

// ResolveOutputs recursively replaces all output(T) and promise(T) types in the input type with their element type.
func ResolveOutputs(t Type) Type {
	resolved, _ := resolveEventuals(t, true)
	return resolved
}

// PromiseType represents eventual values that do not carry dependency information (e.g invoke return values)
type PromiseType struct {
	// ElementType is the element type of the promise.
	ElementType Type

	s string
}

// The set of promise types, indexed by element type.
var promiseTypes = map[Type]*PromiseType{}

// NewPromiseType creates a new promise type with the given element type after replacing any promise types within
// the element type with their respective element types.
func NewPromiseType(elementType Type) *PromiseType {
	elementType = ResolvePromises(elementType)
	if t, ok := promiseTypes[elementType]; ok {
		return t
	}

	t := &PromiseType{ElementType: elementType}
	promiseTypes[elementType] = t
	return t
}

func (*PromiseType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *PromiseType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	element, diagnostics := t.ElementType.Traverse(traverser)
	return NewPromiseType(element.(Type)), diagnostics
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A promise(T) is assignable
// from values of type any, promise(U), and U, where T is assignable from U.
func (t *PromiseType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t {
		return true
	}

	if other, ok := src.(*PromiseType); ok {
		return t.ElementType.AssignableFrom(other.ElementType)
	}

	return t.ElementType.AssignableFrom(src)
}

func (t *PromiseType) String() string {
	if t.s == "" {
		t.s = fmt.Sprintf("promise(%v)", t.ElementType)
	}
	return t.s
}

func (t *PromiseType) isType() {}

// ResolvePromises recursively replaces all promise(T) types in the input type with their element type.
func ResolvePromises(t Type) Type {
	resolved, _ := resolveEventuals(t, false)
	return resolved
}

// MapType represents maps from strings to particular element types.
type MapType struct {
	// ElementType is the element type of the map.
	ElementType Type

	s string
}

// The set of map types, indexed by element type.
var mapTypes = map[Type]*MapType{}

// NewMapType creates a new map type with the given element type.
func NewMapType(elementType Type) *MapType {
	if t, ok := mapTypes[elementType]; ok {
		return t
	}

	t := &MapType{ElementType: elementType}
	mapTypes[elementType] = t
	return t
}

func (t *MapType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	_, keyType := GetTraverserKey(traverser)

	var diagnostics hcl.Diagnostics
	if !inputType(StringType).AssignableFrom(keyType) {
		diagnostics = hcl.Diagnostics{unsupportedMapKey(traverser.SourceRange())}
	}
	return t.ElementType, diagnostics
}

func (*MapType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A map(T) is assignable
// from values of type any, map(U), and U, where T is assignable from U.
func (t *MapType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t {
		return true
	}

	other, ok := src.(*MapType)
	return ok && t.ElementType.AssignableFrom(other.ElementType)
}

func (t *MapType) String() string {
	if t.s == "" {
		t.s = fmt.Sprintf("map(%v)", t.ElementType)
	}
	return t.s
}

func (*MapType) isType() {}

// ArrayType represents arrays of particular element types.
type ArrayType struct {
	// ElementType is the element type of the array.
	ElementType Type

	s string
}

// The set of array types, indexed by element type.
var arrayTypes = map[Type]*ArrayType{}

// NewArrayType creates a new array type with the given element type.
func NewArrayType(elementType Type) *ArrayType {
	if t, ok := arrayTypes[elementType]; ok {
		return t
	}

	t := &ArrayType{ElementType: elementType}
	arrayTypes[elementType] = t
	return t
}

func (*ArrayType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *ArrayType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	_, indexType := GetTraverserKey(traverser)

	var diagnostics hcl.Diagnostics
	if !inputType(NumberType).AssignableFrom(indexType) {
		diagnostics = hcl.Diagnostics{unsupportedArrayIndex(traverser.SourceRange())}
	}
	return t.ElementType, diagnostics
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A array(T) is assignable
// from values of type any, array(U), and U, where T is assignable from U.
func (t *ArrayType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t {
		return true
	}

	other, ok := src.(*ArrayType)
	return ok && t.ElementType.AssignableFrom(other.ElementType)
}

func (t *ArrayType) String() string {
	if t.s == "" {
		t.s = fmt.Sprintf("array(%v)", t.ElementType)
	}
	return t.s
}

func (*ArrayType) isType() {}

// UnionType represents values that may be any one of a specified set of types.
type UnionType struct {
	// ElementTypes are the allowable types for the union type.
	ElementTypes []Type

	s string
}

// The set of union types, indexed by string representation.
var unionTypes = map[string]*UnionType{}

// NewUnionType creates a new union type with the given element types. Any element types that are union types are
// replaced with their element types.
func NewUnionType(type1, type2 Type, rest ...Type) Type {
	var elementTypes []Type
	if t, ok := type1.(*UnionType); ok {
		elementTypes = append(elementTypes, t.ElementTypes...)
	} else {
		elementTypes = append(elementTypes, type1)
	}
	if t, ok := type2.(*UnionType); ok {
		elementTypes = append(elementTypes, t.ElementTypes...)
	} else {
		elementTypes = append(elementTypes, type2)
	}
	for _, tn := range rest {
		if t, ok := tn.(*UnionType); ok {
			elementTypes = append(elementTypes, t.ElementTypes...)
		} else {
			elementTypes = append(elementTypes, tn)
		}
	}

	sort.Slice(elementTypes, func(i, j int) bool {
		return elementTypes[i].String() < elementTypes[j].String()
	})

	dst := 0
	for src := 0; src < len(elementTypes); {
		for src < len(elementTypes) && elementTypes[src] == elementTypes[dst] {
			src++
		}
		dst++

		if src < len(elementTypes) {
			elementTypes[dst] = elementTypes[src]
		}
	}
	elementTypes = elementTypes[:dst]

	if len(elementTypes) == 1 {
		return elementTypes[0]
	}

	t := &UnionType{ElementTypes: elementTypes}
	if t, ok := unionTypes[t.String()]; ok {
		return t
	}
	unionTypes[t.String()] = t
	return t
}

func (*UnionType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *UnionType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	// TODO(pdg): produce the union of the results of Traverse on each element type?
	return AnyType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A union(T_0, ..., T_n)
// from values of type any, union(U_0, ..., U_M) where all of U_0 through U_M are assignable to some type in
// (T_0, ..., T_N), and V, where V is assignable to at least one of (T_0, ..., T_N).
func (t *UnionType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t {
		return true
	}
	if other, ok := src.(*UnionType); ok {
		for _, u := range other.ElementTypes {
			if !t.AssignableFrom(u) {
				return false
			}
		}
		return true
	}
	for _, t := range t.ElementTypes {
		if t.AssignableFrom(src) {
			return true
		}
	}
	return false
}

func (t *UnionType) String() string {
	if t.s == "" {
		elements := make([]string, len(t.ElementTypes))
		for i, e := range t.ElementTypes {
			elements[i] = e.String()
		}
		t.s = fmt.Sprintf("union(%s)", strings.Join(elements, ", "))
	}
	return t.s
}

func (*UnionType) isType() {}

// ObjectType represents schematized maps from strings to particular types.
type ObjectType struct {
	// Properties records the types of the object's properties.
	Properties map[string]Type

	s string
}

// The set of object types, indexed by string representation.
var objectTypes = map[string]*ObjectType{}

// NewObjectType creates a new object type with the given properties.
func NewObjectType(properties map[string]Type) *ObjectType {
	t := &ObjectType{Properties: properties}
	if t, ok := objectTypes[t.String()]; ok {
		return t
	}
	objectTypes[t.String()] = t
	return t
}

func (*ObjectType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *ObjectType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	key, keyType := GetTraverserKey(traverser)

	if !inputType(StringType).AssignableFrom(keyType) {
		return AnyType, hcl.Diagnostics{unsupportedObjectProperty(traverser.SourceRange())}
	}

	if key == cty.DynamicVal {
		return AnyType, nil
	}

	propertyName := key.AsString()
	propertyType, hasProperty := t.Properties[propertyName]
	if !hasProperty {
		return AnyType, hcl.Diagnostics{unknownObjectProperty(propertyName, traverser.SourceRange())}
	}
	return propertyType, nil
}

// AssignableFrom returns true if this type is assignable from the indicated source type.
// An object({K_0 = T_0, ..., K_N = T_N}) is assignable from any or
// U = object({K_0 = U_0, ... K_M = U_M}), where T_I is assignable from U[K_I] for all I from 0 to N.
func (t *ObjectType) AssignableFrom(src Type) bool {
	if src == AnyType || src == t {
		return true
	}

	srcObject, ok := src.(*ObjectType)
	if !ok {
		return false
	}
	for k, dst := range t.Properties {
		if !dst.AssignableFrom(srcObject.Properties[k]) {
			return false
		}
	}
	return true
}

func (t *ObjectType) String() string {
	if t.s == "" {
		var properties []string
		for k, v := range t.Properties {
			properties = append(properties, fmt.Sprintf("%s = %v", k, v))
		}
		sort.Strings(properties)

		t.s = fmt.Sprintf("object({%s})", strings.Join(properties, ", "))
	}
	return t.s
}

func (*ObjectType) isType() {}

// TokenType represents a type that is named by a type token.
type TokenType struct {
	// Token is the type's Pulumi type token.
	Token string

	s string
}

// The set of token types, indexed by token.
var tokenTypes = map[string]*TokenType{}

// GetTokenType fetches the token type for the given token.
func GetTokenType(token string) (*TokenType, bool) {
	t, ok := tokenTypes[token]
	return t, ok
}

// NewTokenType creates a new token type with the given token and underlying type.
func NewTokenType(token string) (*TokenType, error) {
	if _, ok := tokenTypes[token]; ok {
		return nil, errors.Errorf("token type %s is already defined", token)
	}

	t := &TokenType{Token: token}
	tokenTypes[token] = t
	return t, nil
}

func (*TokenType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *TokenType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return AnyType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A token(name) is assignable
// from any or token(name).
func (t *TokenType) AssignableFrom(src Type) bool {
	return src == AnyType || src == t
}

func (t *TokenType) String() string {
	if t.s == "" {
		t.s = fmt.Sprintf("token(%s)", t.Token)
	}
	return t.s
}

func (*TokenType) isType() {}

// IsOptionalType returns true if t is an optional type.
func IsOptionalType(t Type) bool {
	return t.AssignableFrom(nil)
}

func isEventualType(t Type) (Type, bool) {
	switch t := t.(type) {
	case *OutputType:
		return t.ElementType, true
	case *PromiseType:
		return t.ElementType, true
	default:
		return nil, false
	}
}

func liftOperationType(resultType Type, arguments ...Expression) Type {
	var transform typeTransform
	for _, arg := range arguments {
		_, t := resolveEventuals(arg.Type(), true)
		if transform == makeIdentity || transform != makeOutput {
			transform = t
		}
	}
	return transform.do(resultType)
}

var inputTypes = map[Type]Type{}

func inputType(t Type) Type {
	if t == AnyType || t == nil {
		return t
	}
	if input, ok := inputTypes[t]; ok {
		return input
	}

	var src Type
	switch t := t.(type) {
	case *OptionalType:
		src = NewOptionalType(inputType(t.ElementType))
	case *OutputType:
		return t
	case *PromiseType:
		src = NewPromiseType(inputType(t.ElementType))
	case *MapType:
		src = NewMapType(inputType(t.ElementType))
	case *ArrayType:
		src = NewArrayType(inputType(t.ElementType))
	case *UnionType:
		elementTypes := make([]Type, len(t.ElementTypes))
		for i, t := range t.ElementTypes {
			elementTypes[i] = inputType(t)
		}
		src = NewUnionType(elementTypes[0], elementTypes[1], elementTypes[2:]...)
	case *ObjectType:
		properties := map[string]Type{}
		for k, t := range t.Properties {
			properties[k] = inputType(t)
		}
		src = NewObjectType(properties)
	default:
		src = t
	}

	input := NewUnionType(src, NewOutputType(src))
	inputTypes[t] = input
	return input
}
