package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

type ConversionKind = model.ConversionKind

// Type represents a datatype in the Pulumi Schema. Types created by this package are identical if they are
// equal values.
type Type = model.Type

const NoConversion = model.NoConversion

const UnsafeConversion = model.UnsafeConversion

const SafeConversion = model.SafeConversion

var NoneType = model.NoneType

var BoolType = model.BoolType

var IntType = model.IntType

var NumberType = model.NumberType

var StringType = model.StringType

var DynamicType = model.DynamicType

// UnifyTypes chooses the most general type that is convertible from all of the input types.
func UnifyTypes(types ...Type) (safeType Type, unsafeType Type) {
	return model.UnifyTypes(types...)
}

