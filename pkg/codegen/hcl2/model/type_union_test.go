package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrettyPrintingUnionType(t *testing.T) {
	t.Parallel()
	union := NewUnionType(StringType, IntType)
	pretty := union.Pretty().String()
	assert.Equal(t, "int | string", pretty)
}

func TestPrettyPrintingNestedUnionType(t *testing.T) {
	t.Parallel()
	union := NewUnionType(StringType, NewUnionType(IntType, BoolType))
	pretty := union.Pretty().String()
	assert.Equal(t, "bool | int | string", pretty)
}

func TestPrettyPrintingSelfReferencingUnionType(t *testing.T) {
	t.Parallel()
	union := &UnionType{ElementTypes: []Type{
		StringType,
		IntType,
	}}

	union = &UnionType{ElementTypes: []Type{
		StringType,
		&ListType{
			ElementType: &ObjectType{
				Properties: map[string]Type{
					"selfReferences": union,
				},
			},
		},
	}}

	pretty := union.Pretty().String()
	assert.Equal(t, "string | list({ selfReferences: string | int })", pretty)
}
