// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"fmt"

	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Type is a union type that can represent any of the sort of "types" in the system.
type Type struct {
	// one, and only one, of these will be non-nil:
	Primitive   *PrimitiveType         // a simple type (like `string`, `number`, etc).
	Decors      *TypeDecors            // a decorated type; this is either an array or map.
	Unref       *pack.PackageURLString // an unresolved name.
	UninstStack *UninstStack           // a resolved, but uninstantiated, stack reference.
	Stack       *Stack                 // a specific stack type.
	Schema      *Schema                // a specific schema type.
}

func NewPrimitiveType(p *PrimitiveType) *Type {
	return &Type{Primitive: p}
}

func NewAnyType() *Type {
	return NewPrimitiveType(&PrimitiveTypeAny)
}

func NewBoolType() *Type {
	return NewPrimitiveType(&PrimitiveTypeBool)
}

func NewNumberType() *Type {
	return NewPrimitiveType(&PrimitiveTypeNumber)
}

func NewStringType() *Type {
	return NewPrimitiveType(&PrimitiveTypeString)
}

func NewServiceType() *Type {
	return NewPrimitiveType(&PrimitiveTypeService)
}

func NewArrayType(elemType *Type) *Type {
	return &Type{Decors: &TypeDecors{ElemType: elemType}}
}

func NewMapType(keyType, valType *Type) *Type {
	return &Type{Decors: &TypeDecors{KeyType: keyType, ValueType: valType}}
}

func NewStackType(stack *Stack) *Type {
	return &Type{Stack: stack}
}

func NewSchemaType(schema *Schema) *Type {
	return &Type{Schema: schema}
}

func NewUninstStackType(uninst *UninstStack) *Type {
	return &Type{UninstStack: uninst}
}

func NewUnresolvedRefType(ref *pack.PackageURLString) *Type {
	return &Type{Unref: ref}
}

// Name converts the given type into its corresponding friendly name.
func (ty *Type) Name() pack.PackageURLString {
	if ty.Primitive != nil {
		return pack.PackageURLString(*ty.Primitive)
	} else if ty.Stack != nil {
		return pack.PackageURLString(ty.Stack.Name)
	} else if ty.Schema != nil {
		return pack.PackageURLString(ty.Schema.Name)
	} else if ty.Unref != nil {
		return *ty.Unref
	} else if ty.UninstStack != nil {
		return ty.UninstStack.Ref
	} else if ty.Decors != nil {
		// TODO: consider caching these so we don't produce lots of strings.
		if ty.Decors.ElemType != nil {
			return pack.PackageURLString(fmt.Sprintf(string(TypeDecorsArray), ty.Decors.ElemType.Name()))
		} else {
			contract.Assert(ty.Decors.KeyType != nil)
			contract.Assert(ty.Decors.ValueType != nil)
			return pack.PackageURLString(fmt.Sprintf(string(TypeDecorsMap), ty.Decors.KeyType.Name(), ty.Decors.ValueType.Name()))
		}
	} else {
		contract.Failf("Expected this type to have one of primitive, stack, schema, unref, resref, or decors")
		return pack.PackageURLString("")
	}
}

// IsDecors checks whether the Type is decorated.
func (ty *Type) IsDecors() bool {
	return ty.Decors != nil
}

// IsPrimitive checks whether the Type is primitive.
func (ty *Type) IsPrimitive() bool {
	return ty.Primitive != nil
}

// IsStack checks whether the Type represents a bound Stack node.
func (ty *Type) IsStack() bool {
	return ty.Stack != nil
}

// IsSchema checks whether the Type represents a bound Schema node.
func (ty *Type) IsSchema() bool {
	return ty.Schema != nil
}

// IsUninstStack checks whether the Type is a resolved named reference.
func (ty *Type) IsUninstStack() bool {
	return ty.UninstStack != nil
}

// IsUnresolvedRef checks whether the Type is an unresolved named reference.
func (ty *Type) IsUnresolvedRef() bool {
	return ty.Unref != nil
}

// String merely provides a convenient Stringer implementation that fetches a type's name.
func (ty *Type) String() string {
	return string(ty.Name())
}

// TypeDecors is non-nil for arrays and maps, and contains other essential information about them.
type TypeDecors struct {
	ElemType  *Type // the element type, non-nil only for arrays
	KeyType   *Type // the key type, non-nil only for maps
	ValueType *Type // the value type, non-nil only for maps
}

// TypeDecorsFormat is a modifier for arrays and maps.
type TypeDecorsFormat string

const (
	TypeDecorsArray        TypeDecorsFormat = "%v" + TypeDecorsArraySuffix
	TypeDecorsArraySuffix                   = "[]"
	TypeDecorsMap                           = TypeDecorsMapPrefix + "%v" + TypeDecorsMapSeparator + "%v"
	TypeDecorsMapPrefix                     = "map["
	TypeDecorsMapSeparator                  = "]"
)

// PrimitiveType is the name of a primitive type.
type PrimitiveType tokens.Name

// A set of known primitive types.
var (
	PrimitiveTypeAny     PrimitiveType = "any"     // any structure.
	PrimitiveTypeBool    PrimitiveType = "bool"    // a JSON-like boolean (`true` or `false`).
	PrimitiveTypeNumber  PrimitiveType = "number"  // a JSON-like number (integer or floating point).
	PrimitiveTypeService PrimitiveType = "service" // an untyped service reference; at runtime, a URL.
	PrimitiveTypeString  PrimitiveType = "string"  // a JSON-like string.
)

// NewAnyLiteral allocates a fresh AnyLiteral with the given contents.
func NewAnyLiteral(node *Node, any interface{}) AnyLiteral {
	return &anyLiteral{node, any}
}

type anyLiteral struct {
	node *Node
	any  interface{}
}

var _ AnyLiteral = (*anyLiteral)(nil) // ensure anyLiteral implements AnyLiteral.

func (l *anyLiteral) Node() *Node      { return l.node }
func (l *anyLiteral) Type() *Type      { return NewAnyType() }
func (l *anyLiteral) Any() interface{} { return l.any }

// NewBoolLiteral allocates a fresh BoolLiteral with the given contents.
func NewBoolLiteral(node *Node, b bool) BoolLiteral {
	return &boolLiteral{node, b}
}

type boolLiteral struct {
	node *Node
	b    bool
}

var _ BoolLiteral = (*boolLiteral)(nil) // ensure boolLiteral implements BoolLiteral.

func (l *boolLiteral) Node() *Node { return l.node }
func (l *boolLiteral) Type() *Type { return NewBoolType() }
func (l *boolLiteral) Bool() bool  { return l.b }

// NewNumberLiteral allocates a fresh NumberLiteral with the given contents.
func NewNumberLiteral(node *Node, n float64) NumberLiteral {
	return &numberLiteral{node, n}
}

type numberLiteral struct {
	node *Node
	n    float64
}

var _ NumberLiteral = (*numberLiteral)(nil) // ensure numberLiteral implements NumberLiteral.

func (l *numberLiteral) Node() *Node     { return l.node }
func (l *numberLiteral) Type() *Type     { return NewNumberType() }
func (l *numberLiteral) Number() float64 { return l.n }

// NewStringLiteral allocates a fresh StringLiteral with the given contents.
func NewStringLiteral(node *Node, s string) StringLiteral {
	return &stringLiteral{node, s}
}

type stringLiteral struct {
	node *Node
	s    string
}

var _ StringLiteral = (*stringLiteral)(nil) // ensure stringLiteral implements StringLiteral.

func (l *stringLiteral) Node() *Node    { return l.node }
func (l *stringLiteral) Type() *Type    { return NewStringType() }
func (l *stringLiteral) String() string { return l.s }

// NewServiceLiteral allocates a fresh ServiceLiteral with the given contents.
func NewServiceLiteral(node *Node, sref *ServiceRef) ServiceLiteral {
	return &serviceLiteral{node, sref}
}

type serviceLiteral struct {
	node *Node
	sref *ServiceRef
}

var _ ServiceLiteral = (*serviceLiteral)(nil) // ensure serviceLiteral implements ServiceLiteral.

func (l *serviceLiteral) Node() *Node { return l.node }

// TODO should Type return Stack?

func (l *serviceLiteral) Type() *Type          { return NewServiceType() }
func (l *serviceLiteral) Service() *ServiceRef { return l.sref }

// NewArrayLiteral allocates a fresh ArrayLiteral with the given contents.
func NewArrayLiteral(node *Node, elemType *Type, arr []Literal) ArrayLiteral {
	return &arrayLiteral{node, elemType, arr}
}

type arrayLiteral struct {
	node     *Node
	elemType *Type
	arr      []Literal
}

var _ ArrayLiteral = (*arrayLiteral)(nil) // ensure arrayLiteral implements ArrayLiteral.

func (l *arrayLiteral) Node() *Node      { return l.node }
func (l *arrayLiteral) Type() *Type      { return NewArrayType(l.elemType) }
func (l *arrayLiteral) ElemType() *Type  { return l.elemType }
func (l *arrayLiteral) Array() []Literal { return l.arr }

// NewMapLiteral allocates a fresh MapLiteral with the given contents.
func NewMapLiteral(node *Node, keyType *Type, valType *Type, keys []Literal, vals []Literal) MapLiteral {
	return &mapLiteral{node, keyType, valType, keys, vals}
}

type mapLiteral struct {
	node    *Node
	keyType *Type
	valType *Type
	keys    []Literal
	vals    []Literal
}

var _ MapLiteral = (*mapLiteral)(nil) // ensure mapLiteral implements MapLiteral.

func (l *mapLiteral) Node() *Node       { return l.node }
func (l *mapLiteral) Type() *Type       { return NewMapType(l.keyType, l.valType) }
func (l *mapLiteral) KeyType() *Type    { return l.keyType }
func (l *mapLiteral) ValueType() *Type  { return l.valType }
func (l *mapLiteral) Keys() []Literal   { return l.keys }
func (l *mapLiteral) Values() []Literal { return l.vals }

// NewSchemaLiteral allocates a fresh SchemaLiteral with the given contents.
func NewSchemaLiteral(node *Node, schema *Schema, props LiteralPropertyBag) SchemaLiteral {
	return &schemaLiteral{node, schema, props}
}

type schemaLiteral struct {
	node   *Node
	schema *Schema
	props  LiteralPropertyBag
}

var _ SchemaLiteral = (*schemaLiteral)(nil) // ensure schemaLiteral implements SchemaLiteral.

func (l *schemaLiteral) Node() *Node                    { return l.node }
func (l *schemaLiteral) Type() *Type                    { return NewSchemaType(l.schema) }
func (l *schemaLiteral) Properties() LiteralPropertyBag { return l.props }
