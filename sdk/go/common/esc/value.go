// Copyright 2023, Pulumi Corporation.
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

package esc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/esc/internal/util"
	"golang.org/x/exp/maps"
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
		Value   json.RawMessage `json:"value,omitempty"`
		Secret  bool            `json:"secret,omitempty"`
		Unknown bool            `json:"unknown,omitempty"`
		Trace   Trace           `json:"trace"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	v.Secret = raw.Secret
	v.Unknown = raw.Unknown
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

// FromJSON converts a plain-old-JSON value (i.e. a value of type nil, bool, json.Number, string, []any, or
// map[string]any) into a Value.
func FromJSON(v any, secret bool) (Value, error) {
	return fromJSON("", v, secret)
}

func fromJSON(path string, v any, secret bool) (Value, error) {
	switch v := v.(type) {
	case nil:
		return Value{}, nil
	case bool:
		if secret {
			return NewSecret(v), nil
		}
		return NewValue(v), nil
	case json.Number:
		if secret {
			return NewSecret(v), nil
		}
		return NewValue(v), nil
	case string:
		if secret {
			return NewSecret(v), nil
		}
		return NewValue(v), nil
	case []any:
		vs := make([]Value, len(v))
		for i, v := range v {
			vv, err := fromJSON(fmt.Sprintf("[%v]", i), v, secret)
			if err != nil {
				return Value{}, err
			}
			vs[i] = vv
		}
		if secret {
			return NewSecret(vs), nil
		}
		return NewValue(vs), nil
	case map[string]any:
		keys := maps.Keys(v)
		sort.Strings(keys)
		vs := make(map[string]Value, len(keys))
		for _, k := range keys {
			vv, err := fromJSON(util.JoinKey(path, k), v[k], secret)
			if err != nil {
				return Value{}, err
			}
			vs[k] = vv
		}
		if secret {
			return NewSecret(vs), nil
		}
		return NewValue(vs), nil
	default:
		return Value{}, fmt.Errorf("%v: unsupported value of type %T", path, v)
	}
}

// ToJSON converts a Value into a plain-old-JSON value (i.e. a value of type nil, bool, json.Number, string, []any, or
// map[string]any). If redact is true, secrets are replaced with [secret].
func (v Value) ToJSON(redact bool) any {
	if v.Secret && redact {
		return "[secret]"
	}
	if v.Unknown {
		return "[unknown]"
	}

	switch pv := v.Value.(type) {
	case []Value:
		a := make([]any, len(pv))
		for i, v := range pv {
			a[i] = v.ToJSON(redact)
		}
		return a
	case map[string]Value:
		m := make(map[string]any, len(pv))
		for k, v := range pv {
			m[k] = v.ToJSON(redact)
		}
		return m
	default:
		return pv
	}
}

// ToString returns the string representation of this value. If redact is true, secrets are replaced with [secret].
func (v Value) ToString(redact bool) string {
	if v.Secret && redact {
		return "[secret]"
	}
	if v.Unknown {
		return "[unknown]"
	}

	switch pv := v.Value.(type) {
	case bool:
		if pv {
			return "true"
		}
		return "false"
	case json.Number:
		return pv.String()
	case string:
		return pv
	case []Value:
		vals := make([]string, len(pv))
		for i, v := range pv {
			vals[i] = strconv.Quote(v.ToString(redact))
		}
		return strings.Join(vals, ",")
	case map[string]Value:
		keys := maps.Keys(pv)
		sort.Strings(keys)

		pairs := make([]string, len(pv))
		for i, k := range keys {
			pairs[i] = fmt.Sprintf("%q=%q", k, pv[k].ToString(redact))
		}
		return strings.Join(pairs, ",")
	default:
		return ""
	}
}

// String is shorthand for ToString(true).
func (v Value) String() string {
	return v.ToString(true)
}
