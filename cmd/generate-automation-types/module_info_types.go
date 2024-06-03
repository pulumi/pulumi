package main

type FieldType interface {
	isFieldType()
}

type StringType struct{}

func (s StringType) isFieldType() {}

type IntType struct{}

func (i IntType) isFieldType() {}

type FloatType struct{}

func (f FloatType) isFieldType() {}

type BoolType struct{}

func (b BoolType) isFieldType() {}

type AnyType struct{}

func (a AnyType) isFieldType() {}

type ListType struct {
	ElementType FieldType
}

func (l ListType) isFieldType() {}

type MapType struct {
	KeyType   FieldType
	ValueType FieldType
}

func (m MapType) isFieldType() {}

type RefType struct {
	Reference string
}

func (r RefType) isFieldType() {}
