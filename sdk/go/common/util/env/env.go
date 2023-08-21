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
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
type Env interface {
	GetString(val StringValue) string
	GetBool(val BoolValue) bool
	GetInt(val IntValue) int
}

// Create a new strongly typed Env from an untyped Store.
func NewEnv(store Store) Env {
	return env{store}
}

type env struct{ s Store }

func (e env) GetString(val StringValue) string {
	return StringValue{val.withStore(e.s)}.Value()
}

func (e env) GetBool(val BoolValue) bool {
	return BoolValue{val.withStore(e.s)}.Value()
}

func (e env) GetInt(val IntValue) int {
	return IntValue{val.withStore(e.s)}.Value()
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
	defaultValue string

	// If err is non-nil, then this value will always fail validation.
	err error
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

// Add a default value to the variable.
//
// If f returns a non-nil error, then the variable will fail validation.
func DefaultF(f func() (string, error)) Option {
	return func(opts *options) {
		s, err := f()
		opts.defaultValue = s
		opts.err = err
	}
}

// The value of a environmental variable.
//
// In general, `Value`s should only be used in collections. For specific values, used the
// typed version (StringValue, BoolValue).
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

	Type() string

	Validate() ValidateError

	// Get the value, or the default value as appropriate. If "" is returned, a type
	// specific default value should be provided.
	get() string

	// This is the inner validate method that should be implemented by Value*
	// types. Only [value] should implement [Validate] directly.
	validate(input string) ValidateError

	// set the associated variable for the value. This is necessary since Value and Var
	// are inherently cyclical.
	setVar(Var)
	// A type correct formatting for the value. This is used for display purposes and
	// should not be quoted.
	//
	// This function should not be called directly, except by [fmtValue].
	formattedValue() string
}

type ValidateError struct {
	Warning error
	Error   error
}

// An implementation helper for Value. New Values should be a typed wrapper around *value.
type value struct {
	variable Var
	store    Store
}

func (v value) Validate() ValidateError {
	if err := v.variable.options.err; err != nil {
		// If we have a preset error, fail with that.
		return ValidateError{Error: err}
	}
	value := v.get()
	_, specified := v.Underlying()
	var warn error
	if m := v.missingPrerequs(); m != "" && specified {
		warn = fmt.Errorf("cannot set because %q required %q", v.variable.name, m)
	}

	// We validate if the user specified a value, or if we set a default value.
	var validation ValidateError
	if specified || value != "" {
		validation = v.variable.Value.validate(value)
	}
	if validation.Warning == nil && warn != nil {
		validation.Warning = warn
	}
	return validation
}

func (v value) withStore(store Store) *value {
	v.store = store // This is non-mutating since `v` is taken by value.
	return &v
}

func (v value) hasDefault() bool { return v.variable.options.defaultValue != "" }

func (v value) String() string {
	_, present := v.Underlying()
	if !present {
		msg := "unset"
		if v.hasDefault() {
			msg = "default " + fmtValue(v)
		}
		return msg
	}

	if m := v.missingPrerequs(); m != "" {
		return fmt.Sprintf("needs %s (%s)", m, fmtValue(v))
	}
	return fmtValue(v)
}

func fmtValue(v value) string {
	if v.variable.options.secret {
		return "[secret]"
	}
	return v.variable.Value.formattedValue()
}

func (v *value) setVar(variable Var) {
	v.variable = variable
}

func (v value) Var() Var {
	return v.variable
}

// Get the value that will be used.
func (v value) get() string {
	defValue := v.variable.options.defaultValue
	if v.missingPrerequs() != "" {
		return defValue
	}
	val, ok := v.Underlying()
	if ok {
		return val
	}
	return defValue
}

func (v value) Underlying() (string, bool) {
	s := v.store
	if s == nil {
		s = Global
	}
	found, ok := s.Raw(v.Var().Name())
	if !ok && v.hasDefault() {
		found = v.variable.options.defaultValue
	}
	return found, ok
}

func (v value) missingPrerequs() string {
	for _, p := range v.variable.options.prerequs {
		if !p.Value() {
			return p.Var().Name()
		}
	}
	return ""
}

// A string retrieved from the environment.
type StringValue struct{ *value }

func (StringValue) Type() string { return "string" }

func (s StringValue) formattedValue() string {
	return fmt.Sprintf("%#v", s.Value())
}

func (StringValue) validate(string) ValidateError { return ValidateError{} }

// The string value of the variable.
//
// If the variable is unset, "" is returned.
func (s StringValue) Value() string { return s.get() }

// A boolean retrieved from the environment.
type BoolValue struct{ *value }

func (BoolValue) Type() string { return "bool" }

func (b BoolValue) formattedValue() string {
	return fmt.Sprintf("%#v", b.Value())
}

func (BoolValue) validate(v string) ValidateError {
	if cmdutil.IsTruthy(v) || v == "0" || strings.EqualFold(v, "false") {
		return ValidateError{}
	}
	return ValidateError{
		Warning: fmt.Errorf("%#v is falsy, but doesn't look like a boolean", v),
	}
}

// The boolean value of the variable.
//
// If the variable is unset, false is returned.
func (b BoolValue) Value() bool { return cmdutil.IsTruthy(b.get()) }

// An integer retrieved from the environment.
type IntValue struct{ *value }

func (IntValue) Type() string { return "int" }

func (IntValue) validate(v string) ValidateError {
	_, err := strconv.ParseInt(v, 10, 64)
	return ValidateError{
		Error: err,
	}
}

// The integer value of the variable.
//
// If the variable is unset or not parsable, 0 is returned.
func (i IntValue) Value() int {
	v := i.get()
	if v == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return int(parsed)
}

func (i IntValue) formattedValue() string {
	return fmt.Sprintf("%#v", i.Value())
}

func setVar(val Value, variable Var) Value {
	variable.Value = val
	val.setVar(variable)
	envVars = append(envVars, variable)
	return val
}

// Declare a new environmental value.
//
// `name` is the runtime name of the variable. Unless `NoPrefix` is passed, name is
// pre-appended with `Prefix`. For example, a variable named "FOO" would be set by
// declaring "PULUMI_FOO=val" in the enclosing environment.
//
// `description` is the string description of what the variable does.
func String(name, description string, opts ...Option) StringValue {
	var options options
	for _, opt := range opts {
		opt(&options)
	}
	val := StringValue{&value{}}
	variable := Var{
		name:        name,
		Description: description,
		options:     options,
	}
	return setVar(val, variable).(StringValue)
}

// Declare a new environmental value of type bool.
//
// `name` is the runtime name of the variable. Unless `NoPrefix` is passed, name is
// pre-appended with `Prefix`. For example, a variable named "FOO" would be set by
// declaring "PULUMI_FOO=1" in the enclosing environment.
//
// `description` is the string description of what the variable does.
func Bool(name, description string, opts ...Option) BoolValue {
	var options options
	for _, opt := range opts {
		opt(&options)
	}
	val := BoolValue{&value{}}
	variable := Var{
		name:        name,
		Description: description,
		options:     options,
	}
	return setVar(val, variable).(BoolValue)
}

// Declare a new environmental value of type integer.
//
// `name` is the runtime name of the variable. Unless `NoPrefix` is passed, name is
// pre-appended with `Prefix`. For example, a variable named "FOO" would be set by
// declaring "PULUMI_FOO=1" in the enclosing environment.
//
// `description` is the string description of what the variable does.
func Int(name, description string, opts ...Option) IntValue {
	var options options
	for _, opt := range opts {
		opt(&options)
	}
	val := IntValue{&value{}}
	variable := Var{
		name:        name,
		Description: description,
		options:     options,
	}
	return setVar(val, variable).(IntValue)
}
