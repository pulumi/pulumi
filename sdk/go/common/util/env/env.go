// Copyright 2016-2022, Pulumi Corporation.
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

// A small library for creating typed, consistent and documented environmental variable
// accesses.
//
// Declaring a variable is as simple as declaring a module level constant.
//
//	var Var = env.Bool("VAR", "A boolean variable")
//
// Typed values can be retrieved by calling `Var.Value()`.
package env

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"sync"
)

// Store holds a collection of key, value? pairs.
//
// Store acts like the environment that values are drawn from.
type Store interface {
	// Retrieve a raw value from the Store. If the value is not present, "", false should
	// be returned.
	Raw(key string) (string, bool)
}

// A strongly typed environment.
//
// Env must be comparable.
type Env interface {
	GetString(val StringValue) string
	GetBool(val BoolValue) bool
	GetInt(val IntValue) int

	// Set the underlying environment of the value to the caller.
	String(StringValue) StringValue
	// Set the underlying environment of the value to the caller.
	Bool(BoolValue) BoolValue
	// Set the underlying environment of the value to the caller.
	Int(IntValue) IntValue
}

// Create a new strongly typed Env from an untyped Store.
func NewEnv(store Store) Env {
	if reflect.TypeOf(store).Comparable() {
		return env{store}
	}
	return &env{store}
}

type env struct{ s Store }

func (e env) String(val StringValue) StringValue {
	return StringValue{val.withStore(e.s)}
}

func (e env) Bool(val BoolValue) BoolValue {
	return BoolValue{val.withStore(e.s)}
}

func (e env) Int(val IntValue) IntValue {
	return IntValue{val.withStore(e.s)}
}

func (e env) GetString(val StringValue) string {
	return e.String(val).Value()
}

func (e env) GetBool(val BoolValue) bool {
	return e.Bool(val).Value()
}

func (e env) GetInt(val IntValue) int {
	return e.Int(val).Value()
}

type envStore struct{}

func (envStore) Raw(key string) (string, bool) {
	return os.LookupEnv(key)
}

type MapStore map[string]string

func (m MapStore) Raw(key string) (string, bool) {
	v, ok := m[key]
	return v, ok
}

// The global store of values that Value.Value() uses.
//
// Setting this value is not thread safe, and should be restricted to testing.
var Global Store = envStore{}

// An environmental variable.
type Var struct {
	name        string
	Value       Value
	Description string
	options     options
}

// The default prefix for a environmental variable.
const Prefix = "PULUMI_"

// The name of an environmental variable.
//
// Name accounts for prefix if appropriate.
func (v Var) Name() string {
	return v.options.name(v.name)
}

// The list of variables that a must be truthy for `v` to be set.
func (v Var) Requires() []BoolValue { return v.options.prerequs }

var envVars []Var

// Variables is a list of variables declared.
func Variables() []Var {
	vars := envVars
	sort.SliceStable(vars, func(i, j int) bool {
		return vars[i].Name() < vars[j].Name()
	})
	return vars
}

// An Option to configure a environmental variable.
type Option func(*options)

type options struct {
	prerequs []BoolValue
	noPrefix bool
	secret   bool
	// The default value to read in. It will be parsed as if it was provided from a
	// Store.
	defaultValue *defaultValue
}

// A per-store default value.
//
// To access a field inside the defaultValue, you must hold m.
type defaultValue struct {
	m         *sync.Mutex
	generator func(Env) (string, error)
	// Results that have already been computed.
	//
	// Results are computed once per store and on demand.
	results map[Env]struct {
		result string
		err    error
	}
}

// Turn a function into a default value.
func deferF(f func(Env) (string, error)) *defaultValue {
	return &defaultValue{generator: f, m: new(sync.Mutex)}
}

// Retrieve the value on a store.
func (d *defaultValue) get(s Env) (string, error) {
	// The default default value is the empty string.
	if d == nil {
		return "", nil
	}
	d.m.Lock()
	defer d.m.Unlock()
	if d.results == nil {
		d.results = map[Env]struct {
			result string
			err    error
		}{}
	}
	v, computed := d.results[s]
	if !computed {
		v.result, v.err = d.generator(s)
		d.results[s] = v
	}
	return v.result, v.err
}

func (o options) name(underlying string) string {
	if o.noPrefix {
		return underlying
	}
	return Prefix + underlying
}

// Needs indicates that a variable can only be set if `val` is truthy.
func Needs(val BoolValue) Option {
	return func(o *options) {
		o.prerequs = append(o.prerequs, val)
	}
}

// NoPrefix indicates that a variable should not have the default prefix applied.
func NoPrefix(opts *options) {
	opts.noPrefix = true
}

// Secret indicates that the value should not be displayed in plaintext.
func Secret(opts *options) {
	opts.secret = true
}

// DefaultF allows the user to specify a default value for the variable, if unset.
//
// Because default values can depend on other default values, all accesses should use the
// parameterized store.
//
// If f returns a non-nil error, then the variable will fail validation and have a zero
// value.
func DefaultF(f func(Env) (string, error)) Option {
	return func(opts *options) {
		opts.defaultValue = deferF(f)
	}
}

// Set a static default value.
func Default(value string) Option {
	return DefaultF(func(Env) (string, error) { return value, nil })
}

// The value of a environmental variable.
//
// In general, `Value`s should only be used in collections. For specific values, used the
// typed version (StringValue, BoolValue, IntValue).
//
// Every implementer of Value also includes a `Value() T` method that returns a typed
// representation of the value.
type Value interface {
	fmt.Stringer

	// Retrieve the underlying string value associated with the variable.
	//
	// If the variable was not set, ("", false) is returned.
	Underlying() (string, bool)

	// Retrieve the Var associated with this value.
	Var() Var

	// A string describing the type of the value.
	//
	// This should be used for display purposes.
	Type() string

	// Validate that a Value was configured with a type appropriate input.
	//
	// STRING_IN="123" is fine for a StringValue.
	// INT_INT="abc" will fail validation on a IntValue.
	Validate() ValidateError

	// set the associated variable for the value. This is necessary since Value and Var
	// are inherently cyclical.
	setVar(Var)
}

// Encode type specific information about a value.
type valueT[T any] interface {
	// Format a value of T for user display.
	format(T) string
	// Validate that a string can be converted to T.
	validate(string) ValidateError
	// Parse a string to a T. If unable to parse, then an appropriate zero value
	// should be used.
	parse(string) T
	// Return a type string that describes T.
	Type() string
}

type ValidateError struct {
	Warning error
	Error   error
}

// An implementation helper for Value. New Values should be a typed wrapper around *value.
type value[L any, T valueT[L]] struct {
	valueT   T
	variable Var
	store    Store
}

func (v value[L, T]) Value() L {
	return v.valueT.parse(v.get())
}

func (v value[L, T]) Type() string {
	return v.valueT.Type()
}

func (v value[L, T]) Validate() ValidateError {
	value := v.get()
	_, specified := v.Underlying()
	var warn error
	if m := v.missingPrerequs(); m != "" && specified {
		warn = fmt.Errorf("cannot set because %q required %q", v.variable.name, m)
	}

	// We validate if the user specified a value, or if we set a default value.
	var validation ValidateError
	if specified || value != "" {
		validation = v.valueT.validate(value)
	}
	if validation.Warning == nil && warn != nil {
		validation.Warning = warn
	}
	return validation
}

func (v value[L, T]) withStore(store Store) *value[L, T] {
	v.store = store // This is non-mutating since `v` is taken by value.
	return &v
}

func (v value[L, T]) hasDefault() bool { return v.variable.options.defaultValue != nil }

func (v value[L, T]) String() string {
	_, present := v.Underlying()
	if !present {
		msg := "unset"
		if v.hasDefault() {
			msg = "default " + v.format()
		}
		return msg
	}

	if m := v.missingPrerequs(); m != "" {
		return fmt.Sprintf("needs %s (%s)", m, v.format())
	}
	return v.format()
}

func (v value[L, T]) format() string {
	if v.variable.options.secret {
		return "[secret]"
	}
	return v.valueT.format(v.Value())
}

func (v *value[L, T]) setVar(variable Var) {
	v.variable = variable
}

func (v value[L, T]) Var() Var {
	return v.variable
}

// Get the value that will be used.
func (v value[L, T]) get() string {
	defValue := v.variable.options.defaultValue
	if v.missingPrerequs() != "" {
		s, _ := defValue.get(NewEnv(v.getStore()))
		return s
	}
	val, ok := v.Underlying()
	if ok {
		return val
	}
	s, _ := defValue.get(NewEnv(v.getStore()))
	return s
}

func (v value[L, T]) getStore() Store {
	if v.store == nil {
		return Global
	}
	return v.store
}

// Return the value from the underlying store.
func (v value[L, T]) Underlying() (string, bool) {
	s := v.getStore()
	return s.Raw(v.Var().Name())
}

func (v value[L, T]) missingPrerequs() string {
	for _, p := range v.variable.options.prerequs {
		// We need to be careful to get the associated value in the context that
		// we are executing in, and not the global context.
		if !p.withStore(v.getStore()).Value() {
			return p.Var().Name()
		}
	}
	return ""
}

// Create a new value from a V already equipped with an inner value.
func newValue[V Value](name, description string, opts []Option, value V) V {
	var options options
	for _, opt := range opts {
		opt(&options)
	}
	variable := Var{
		name:        name,
		Description: description,
		options:     options,
	}

	// Make the cyclical connection between a value and a variable.
	variable.Value = value
	value.setVar(variable)
	envVars = append(envVars, variable)
	return value
}
