// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"fmt"

	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Object is a value allocated and stored on the heap.  In MuIL's interpreter, all values are heap allocated, since we
// are less concerned about performance of the evaluation (compared to the cost of provisioning cloud resources).
type Object struct {
	Type       symbols.Type // the runtime type of the object.
	Data       Data         // any constant data associated with this object.
	Properties Properties   // the full set of known properties and their values.
}

type Data interface{}                   // literal object data.
type Properties map[tokens.Name]*Object // an object's properties.

// NewObject creates a new empty object of the given type.
func NewObject(t symbols.Type) *Object {
	return &Object{
		Type:       t,
		Properties: make(Properties),
	}
}

// NewPrimitiveObject creates a new primitive object with the given primitive type.
func NewPrimitiveObject(t symbols.Type, data interface{}) *Object {
	return &Object{
		Type: t,
		Data: data,
	}
}

// NewErrorObject creates a new exception with the given message.
func NewErrorObject(message string, args ...interface{}) *Object {
	// TODO: capture a stack trace.
	return NewPrimitiveObject(types.Error, fmt.Sprintf(message, args...))
}

// NewConstantObject returns a new object with the right type and value, based on some constant data.
func NewConstantObject(v interface{}) *Object {
	if v == nil {
		return NewPrimitiveObject(types.Null, nil)
	}
	switch data := v.(type) {
	case bool:
		return NewPrimitiveObject(types.Bool, data)
	case string:
		return NewPrimitiveObject(types.String, data)
	case float64:
		return NewPrimitiveObject(types.Number, data)
	default:
		// TODO: we could support more here (essentially, anything that is JSON serializable).
		contract.Failf("Unrecognized constant data literal: %v", data)
		return nil
	}
}

// Bool asserts that the target is a boolean literal and returns its value.
func (o *Object) Bool() bool {
	contract.Assertf(o.Type == types.Bool, "Expected object type to be Bool; got %v", o.Type)
	contract.Assertf(o.Data != nil, "Expected boolean literal to carry a Data payload; got nil")
	b, ok := o.Data.(bool)
	contract.Assertf(ok, "Expected a boolean literal value for condition expr; conversion failed")
	return b
}

// Number asserts that the target is a numeric literal and returns its value.
func (o *Object) Number() float64 {
	contract.Assertf(o.Type == types.Number, "Expected object type to be Number; got %v", o.Type)
	contract.Assertf(o.Data != nil, "Expected numeric literal to carry a Data payload; got nil")
	n, ok := o.Data.(float64)
	contract.Assertf(ok, "Expected a numeric literal value for condition expr; conversion failed")
	return n
}
