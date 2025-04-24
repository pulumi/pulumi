// Copyright 2016-2025, Pulumi Corporation.
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

package property

import (
	"fmt"
	"strconv"
)

// [fmt.GoStringer] lets a type define the go syntax needed to define it.
//
// If this is implemented well, then you can copy a printed value into your code, which
// makes debugging a lot easier. With that goal in mind, we have chosen to print package
// level constructs prefixed by "property.", since most people debugging with property
// values will not be authors of the property package.
var (
	_ fmt.GoStringer = Value{}
	_ fmt.GoStringer = Map{}
	_ fmt.GoStringer = Array{}
	_ fmt.GoStringer = Null
	_ fmt.GoStringer = Computed
)

func (v Value) GoString() string {
	value := func(s string) string {
		var withSecret, withDependencies string
		if v.isSecret {
			withSecret = ".WithSecret(true)"
		}
		if len(v.dependencies) > 0 {
			withDependencies = fmt.Sprintf(".WithDependencies(%#v)", v.dependencies)
		}
		return fmt.Sprintf("property.New(%s)%s%s", s, withSecret, withDependencies)
	}
	valuef := func(a any) string { return value(fmt.Sprintf("%#v", a)) }
	switch {
	case v.IsBool(), v.IsString(), v.IsComputed(),
		v.IsAsset(), v.IsArchive(), v.IsResourceReference():
		return valuef(v.v)

	// Go doesn't allow New(1), since 1 is a int literal, not a float64 literal.
	//
	// We want to make sure that we always print a valid float64 literal.
	case v.IsNumber():
		n := v.AsNumber()
		s := strconv.FormatFloat(n, 'f', -1, 64)
		if float64(int(n)) == n {
			return value(s + ".0")
		}
		return value(s)

	// Null is normalized to nil, so that Value{} is the same as New(Null).
	case v.IsNull():
		return valuef(Null)

	// [New] accepts both an [Array] or a []Value,

	case v.IsArray():
		a := v.AsArray()
		if len(a.arr) == 0 {
			return valuef(a)
		}
		return valuef(a.arr)
	case v.IsMap():
		m := v.AsMap()
		if len(m.m) == 0 {
			return valuef(m)
		}
		return valuef(v.AsMap().m)
	default:
		panic(fmt.Sprintf("impossible - unknown type %T within a value", v.v))
	}
}

func (a Array) GoString() string {
	if len(a.arr) == 0 {
		return "property.Array{}"
	}
	return fmt.Sprintf("property.NewArray(%#v)", a.arr)
}

func (a Map) GoString() string {
	if len(a.m) == 0 {
		return "property.Map{}"
	}
	return fmt.Sprintf("property.NewMap(%#v)", a.m)
}

func (null) GoString() string { return "property.Null" }

func (computed) GoString() string { return "property.Computed" }
