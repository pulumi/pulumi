// Copyright 2026, Pulumi Corporation.
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

package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// A Cursor is the handle a callback receives on the cursor it runs for. Reads see the values the cursor
// carries plus whatever this callback has set. Writes are visible only within this callback and, once it
// succeeds, downstream of it: a node program's writes become the values the cursor leaves the node with;
// an edge condition's writes apply only if the cursor crosses the edge because of this evaluation. A
// cursor parked on a node is otherwise immutable: the node program is re-run on every up from the values
// the cursor entered with.
type Cursor struct {
	name   string
	values resource.PropertyMap // what the cursor carries

	m       sync.Mutex
	sets    map[string]any
	deleted map[string]bool
	// Outputs may be set only in a node program, where they resolve when the program ends.
	allowOutputs bool
}

func newCursor(name string, values resource.PropertyMap, allowOutputs bool) *Cursor {
	return &Cursor{
		name: name, values: values, sets: map[string]any{}, deleted: map[string]bool{}, allowOutputs: allowOutputs,
	}
}

// Name returns the cursor's name, unique within its workflow.
func (c *Cursor) Name() string { return c.name }

// Set sets key to v. v may be a plain Go value or a pulumi.Input; an Output is only accepted in a node
// program.
func (c *Cursor) Set(key string, v any) {
	c.m.Lock()
	defer c.m.Unlock()
	c.sets[key] = v
	delete(c.deleted, key)
}

// Delete removes key. It is only honored in a node program.
func (c *Cursor) Delete(key string) {
	c.m.Lock()
	defer c.m.Unlock()
	delete(c.sets, key)
	c.deleted[key] = true
}

// lookup returns the value at key as a Go value: a set value first, then the carried one.
func (c *Cursor) lookup(key string) (any, bool) {
	c.m.Lock()
	defer c.m.Unlock()
	if v, ok := c.sets[key]; ok {
		return v, true
	}
	if c.deleted[key] {
		return nil, false
	}
	v, ok := c.values[resource.PropertyKey(key)]
	if !ok {
		return nil, false
	}
	return toGo(v), true
}

// toGo converts a property value to a plain Go value: secrets are unwrapped, numbers are float64,
// objects are map[string]any and arrays []any.
func toGo(v resource.PropertyValue) any {
	switch {
	case v.IsSecret():
		return toGo(v.SecretValue().Element)
	case v.IsOutput():
		return toGo(v.OutputValue().Element)
	case v.IsObject():
		m := make(map[string]any, len(v.ObjectValue()))
		for k, e := range v.ObjectValue() {
			m[string(k)] = toGo(e)
		}
		return m
	case v.IsArray():
		a := make([]any, len(v.ArrayValue()))
		for i, e := range v.ArrayValue() {
			a[i] = toGo(e)
		}
		return a
	case v.IsResourceReference():
		return v.ResourceReferenceValue().URN
	default:
		return v.Mappable()
	}
}

// convert coerces v, a value produced by toGo or passed to Set, to type T. Numbers convert between
// numeric kinds; anything else goes through JSON so that maps convert to structs.
func convert[T any](v any) (T, error) {
	var zero T
	if t, ok := v.(T); ok {
		return t, nil
	}
	want := reflect.TypeFor[T]()
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Type().ConvertibleTo(want) && isNumeric(rv.Kind()) && isNumeric(want.Kind()) {
		return rv.Convert(want).Interface().(T), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return zero, fmt.Errorf("cannot convert %T to %v: %w", v, want, err)
	}
	var t T
	if err := json.Unmarshal(b, &t); err != nil {
		return zero, fmt.Errorf("cannot convert %T to %v: %w", v, want, err)
	}
	return t, nil
}

func isNumeric(k reflect.Kind) bool {
	return k >= reflect.Int && k <= reflect.Float64 && k != reflect.Uintptr
}

// Get returns the value at key as T, coercing numbers between numeric kinds and objects into structs.
func Get[T any](c *Cursor, key string) (T, bool) {
	v, ok := c.lookup(key)
	if !ok {
		var zero T
		return zero, false
	}
	t, err := convert[T](v)
	if err != nil {
		var zero T
		return zero, false
	}
	return t, true
}

// Require is Get, panicking if key is missing or not a T.
func Require[T any](c *Cursor, key string) T {
	v, ok := c.lookup(key)
	if !ok {
		panic(fmt.Sprintf("workflow: cursor %q has no value %q", c.name, key))
	}
	t, err := convert[T](v)
	if err != nil {
		panic(fmt.Sprintf("workflow: cursor %q: %v", c.name, err))
	}
	return t
}
