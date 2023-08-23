// Copyright 2016-2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package env

import "fmt"

// A string retrieved from the environment.
type StringValue struct{ *value[string, stringT] }

type stringT struct{}

func (stringT) Type() string { return "string" }

//nolint:unused
func (stringT) validate(string) ValidateError { return ValidateError{} }

//nolint:unused
func (stringT) parse(s string) string { return s }

//nolint:unused
func (stringT) format(s string) string { return fmt.Sprintf("%#v", s) }

// Declare a new environmental value.
//
// `name` is the runtime name of the variable. Unless `NoPrefix` is passed, name is
// pre-appended with `Prefix`. For example, a variable named "FOO" would be set by
// declaring "PULUMI_FOO=val" in the enclosing environment.
//
// `description` is the string description of what the variable does.
func String(name, description string, opts ...Option) StringValue {
	return newValue(name, description, opts, StringValue{&value[string, stringT]{}})
}
