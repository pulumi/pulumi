// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"strconv"

	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type jsonSerializer struct {
	stack  map[*rt.Object]bool
	intrin *rt.Intrinsic
	e      *evaluator
}

func (s jsonSerializer) serializeJSONProperty(o *rt.Object) (string, *rt.Unwind) {
	if o == nil {
		return "null", nil
	}
	if o.IsNull() {
		return "null", nil
	} else if o.IsBool() {
		if o.BoolValue() {
			return "true", nil
		}
		return "false", nil

	} else if o.IsString() {
		return o.String(), nil
	} else if o.IsNumber() {
		return o.String(), nil
	} else if o.IsArray() {
		return s.serializeJSONArray(o)
	}
	return s.serializeJSONObject(o)
}

func (s jsonSerializer) serializeJSONObject(o *rt.Object) (string, *rt.Unwind) {
	if _, found := s.stack[o]; found {
		return "", s.e.NewException(s.intrin.Tree(), "Cannot JSON serialize an object with cyclic references")
	}
	s.stack[o] = true
	ownProperties := o.Properties().Stable()
	isFirst := true
	final := "{"
	for _, prop := range ownProperties {
		propValuePointer := o.GetPropertyAddr(prop, false, false)
		propValue := propValuePointer.Obj() // TODO: What about getters?
		if propValue == nil {
			continue
		}
		if isFirst {
			final += " "
		} else {
			final += ", "
		}
		isFirst = false
		strP, uw := s.serializeJSONProperty(propValue)
		if uw != nil {
			return "", uw
		}
		final += strconv.Quote(string(prop)) + ": " + strP
	}
	final += "}"
	delete(s.stack, o)
	return final, nil
}

func (s jsonSerializer) serializeJSONArray(o *rt.Object) (string, *rt.Unwind) {
	contract.Assert(o.IsArray()) // expect to be called on an Array
	if _, found := s.stack[o]; found {
		return "", s.e.NewException(s.intrin.Tree(), "Cannot JSON serialize an object with cyclic references")
	}
	s.stack[o] = true

	arr := o.ArrayValue()
	contract.Assert(arr != nil)
	isFirst := true
	final := "["
	for index := 0; index < len(*arr); index++ {
		propValuePointer := (*arr)[index]
		propValue := propValuePointer.Obj() // TODO: What about getters?
		if isFirst {
			final += " "
		} else {
			final += ", "
		}
		isFirst = false
		strP, uw := s.serializeJSONProperty(propValue)
		if uw != nil {
			return "", uw
		}
		final += strP
	}
	final += "]"

	delete(s.stack, o)
	return final, nil
}

// jsonStringify provides JSON serialization of a Lumi object.  This implementation follows a subset of
// https://tc39.github.io/ecma262/2017/#sec-json.stringify without `replacer` and `space` arguments.
func jsonStringify(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(len(args) == 1) // just one arg: the object to stringify
	obj := args[0]
	if obj == nil {
		return rt.NewReturnUnwind(e.alloc.NewString(intrin.Tree(), "{}"))
	}
	s := jsonSerializer{
		map[*rt.Object]bool{},
		intrin,
		e,
	}
	str, uw := s.serializeJSONProperty(obj)
	if uw != nil {
		return uw
	}
	return rt.NewReturnUnwind(e.alloc.NewString(intrin.Tree(), str))
}

func jsonParse(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	return e.NewException(intrin.Tree(), "Not yet implemented - jsonParse")
}
