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

package cobrautil

import (
	"github.com/spf13/pflag"
)

const uuidUnset = "c8f06463-7482-4fdc-9e54-1851290944b8"

type stringPtrValue struct {
	value **string
}

func (v *stringPtrValue) Set(val string) error {
	if val == uuidUnset {
		var empty string
		*v.value = &empty
		return nil
	}
	*v.value = &val
	return nil
}

func (v *stringPtrValue) Type() string {
	return "string"
}

func (v *stringPtrValue) String() string {
	if *v.value == nil {
		return "<nil>"
	}
	return **v.value
}

func NewStringPtrVar(flags *pflag.FlagSet, p **string, name string, usage string) {
	f := flags.VarPF(&stringPtrValue{p}, name, "", usage)
	// This is the _only_ way to get cobra to accept a flag without a value (i.e. --flag instead of --flag=value). Being
	// a string it means we _have_ to take one valid string out the space of arguments, so we use a uuid that is
	// unlikely to ever be used otherwise.
	f.NoOptDefVal = uuidUnset
}
