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

import (
	"fmt"
	"strconv"
)

// An integer retrieved from the environment.
type IntValue struct{ *value[int, intT] }

type intT struct{}

func (intT) Type() string { return "int" }

//nolint:unused
func (intT) format(i int) string { return fmt.Sprintf("%#v", i) }

//nolint:unused
func (intT) validate(v string) ValidateError {
	_, err := strconv.ParseInt(v, 10, 64)
	return ValidateError{
		Error: err,
	}
}

//nolint:unused
func (intT) parse(s string) int {
	i, _ := strconv.ParseInt(s, 10, 64)
	return int(i)
}

// Declare a new environmental value of type integer.
//
// `name` is the runtime name of the variable. Unless `NoPrefix` is passed, name is
// pre-appended with `Prefix`. For example, a variable named "FOO" would be set by
// declaring "PULUMI_FOO=1" in the enclosing environment.
//
// `description` is the string description of what the variable does.
func Int(name, description string, opts ...Option) IntValue {
	return newValue(name, description, opts, IntValue{&value[int, intT]{}})
}
