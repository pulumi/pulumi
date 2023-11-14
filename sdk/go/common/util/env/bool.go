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
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// A boolean retrieved from the environment.
type BoolValue struct{ *value[bool, boolT] }

type boolT struct{}

func (boolT) Type() string { return "bool" }

//nolint:unused
func (boolT) parse(s string) bool { return cmdutil.IsTruthy(s) }

//nolint:unused
func (boolT) format(b bool) string { return fmt.Sprintf("%#v", b) }

//nolint:unused
func (boolT) validate(v string) ValidateError {
	if cmdutil.IsTruthy(v) || v == "0" || strings.EqualFold(v, "false") {
		return ValidateError{}
	}
	return ValidateError{
		Warning: fmt.Errorf("%#v is falsy, but doesn't look like a boolean", v),
	}
}

// Declare a new environmental value of type bool.
//
// `name` is the runtime name of the variable. Unless `NoPrefix` is passed, name is
// pre-appended with `Prefix`. For example, a variable named "FOO" would be set by
// declaring "PULUMI_FOO=1" in the enclosing environment.
//
// `description` is the string description of what the variable does.
func Bool(name, description string, opts ...Option) BoolValue {
	return newValue(name, description, opts, BoolValue{&value[bool, boolT]{}})
}
