// Copyright 2022, Pulumi Corporation.  All rights reserved.

package environments

import (
	"bytes"
	"encoding/json"
)

// ValueType defines the types of concrete values stored inside a Value.
type ValueType interface {
	bool | json.Number | string | []Value | map[string]Value
}

// A Value is the result of evaluating an expression within an environment definition.
type Value struct {
	// Value holds the concrete representation of the value. May be nil, bool, json.Number, string, []Value, or
	// map[string]Value.
	Value any `json:"value,omitempty"`

	// Secret is true if this value is secret.
	Secret bool `json:"secret,omitempty"`

	// Unknown is true if this value is unknown.
	Unknown bool `json:"unknown,omitempty"`

	// Trace holds information about the expression that computed this value and the value (if any) with which it was
	// merged.
	Trace Trace `json:"trace"`
}

// NewValue creates a new value with the given representation.
func NewValue[T ValueType](v T) Value {
	return Value{Value: v}
}

// NewSecret creates a new secret value with the given representation.
func NewSecret[T ValueType](v T) Value {
	return Value{Value: v, Secret: true}
}

// Trace holds information about the expression and base of a value.
type Trace struct {
	// Def is the range of the expression that computed a value.
	Def Range `json:"def"`

	// Base is the base value with which a value was merged.
	Base *Value `json:"base,omitempty"`
}

func (v *Value) UnmarshalJSON(data []byte) error {
	var raw struct {
		Value  json.RawMessage `json:"value,omitempty"`
		Secret bool            `json:"secret,omitempty"`
		Trace  Trace           `json:"trace"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	v.Secret = raw.Secret
	v.Trace = raw.Trace

	if len(raw.Value) != 0 {
		dec := json.NewDecoder(bytes.NewReader([]byte(raw.Value)))
		dec.UseNumber()

		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok {
		case json.Delim('['):
			var arr []Value
			if err := json.Unmarshal([]byte(raw.Value), &arr); err != nil {
				return err
			}
			v.Value = arr
		case json.Delim('{'):
			var obj map[string]Value
			if err := json.Unmarshal([]byte(raw.Value), &obj); err != nil {
				return err
			}
			v.Value = obj
		default:
			v.Value = tok
		}
	}
	return nil
}

// ToJSON converts a Value into a plain-old-JSON value (i.e. a value of type nil, bool, json.Number, string, []any, or
// map[string]any).
func (v Value) ToJSON() any {
	if v.Unknown {
		return "<unknown>"
	}
	switch pv := v.Value.(type) {
	case []Value:
		a := make([]any, len(pv))
		for i, v := range pv {
			a[i] = v.ToJSON()
		}
		return a
	case map[string]Value:
		m := make(map[string]any, len(pv))
		for k, v := range pv {
			m[k] = v.ToJSON()
		}
		return m
	default:
		return pv
	}
}
