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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func isFunction(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil)    // module function
	contract.Assert(len(args) == 1) // just one arg: the object to inquire about functionness
	_, isfunc := args[0].Type().(*symbols.FunctionType)
	ret := e.alloc.NewBool(intrin.Tree(), isfunc)
	return rt.NewReturnUnwind(ret)
}

func dynamicInvoke(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil)    // module function
	contract.Assert(len(args) == 3) // three args: obj, thisArg, and args

	// First ensure the target is a function.
	t := args[0].Type()
	if _, isfunc := t.(*symbols.FunctionType); !isfunc {
		return e.NewIllegalInvokeTargetException(intrin.Tree(), t)
	}

	// Fetch the function stub information (ignoring `this`, since we will pass our own).
	stub := args[0].FunctionValue()

	// Next, simply call through to the evalCall function, which will do all additional verification.
	stub.This = this // adjust this before the call (note this doesn't mutate the source stub; it's by-value).
	obj, uw := e.evalCallFuncStub(intrin.Tree(), stub, args...)
	if uw != nil {
		return uw
	}
	return rt.NewReturnUnwind(obj)
}

func printf(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	var message *rt.Object
	if len(args) >= 1 {
		message = args[0]
	} else {
		message = e.alloc.NewNull(intrin.Tree())
	}

	// TODO[pulumi/lumi#169]: Move this to use a proper ToString() conversion.
	fmt.Printf(message.String())

	return rt.NewReturnUnwind(nil)
}

func sha1hash(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	var str *rt.Object
	if len(args) >= 1 {
		str = args[0]
	} else {
		return e.NewException(intrin.Tree(), "Expected a single argument string.")
	}
	if !str.IsString() {
		return e.NewException(intrin.Tree(), "Expected a single argument string.")
	}

	hasher := sha1.New()
	byts := []byte(str.StringValue())
	hasher.Write(byts)
	sum := hasher.Sum(nil)
	hash := hex.EncodeToString(sum)

	hashObj := e.alloc.NewString(intrin.Tree(), hash)
	return rt.NewReturnUnwind(hashObj)
}

func serializeClosure(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil)    // module function
	contract.Assert(len(args) == 1) // one arg: func

	stub, ok := args[0].TryFunctionValue()
	if !ok {
		return e.NewException(intrin.Tree(), "Expected argument 'func' to be a function value.")
	}
	lambda, ok := stub.Func.(*ast.LambdaExpression)
	if !ok {
		return e.NewException(intrin.Tree(), "Expected argument 'func' to be a lambda expression.")
	}

	// Insert environment variables into a PropertyMap with stable ordering
	envPropMap := rt.NewPropertyMap()
	names := []tokens.Name{}
	vals := map[tokens.Name]*rt.Object{}
	global := e.getModuleGlobals(stub.Module)
	for _, tok := range binder.FreeVars(stub.Func) {
		var name tokens.Name
		var val *rt.Object
		contract.Assertf(tok.Simple() || (tok.HasModule() && tok.HasModuleMember() && !tok.HasClassMember()),
			"Expected free variable to be simple name or reference to top-level module name")
		if tok.Simple() {
			name = tok.Name()
			lv := stub.Env.Lookup(name)
			if lv != nil {
				val = stub.Env.GetValue(lv)
			} else {
				val = global.Properties().Get(rt.PropertyKey(name))
				if val == nil {
					// The variable was not found in the environment, so skip serializing it.
					// This will be true for references to globals which are not known to Lumi but
					// will be available within the runtime environment.
					continue
				}
			}
		} else {
			name = tokens.Name(tok.ModuleMember().Name())
			val = global.Properties().Get(rt.PropertyKey(name))
		}
		names = append(names, name)
		vals[name] = val
	}
	sort.SliceStable(names, func(i, j int) bool { return names[i] < names[j] })
	for _, name := range names {
		envPropMap.Set(rt.PropertyKey(name), vals[name])
	}
	envObj := e.alloc.New(intrin.Tree(), types.Dynamic, envPropMap, nil)

	// Build up the properties for the returned Closure object
	props := rt.NewPropertyMap()
	props.Set("code", rt.NewStringObject(lambda.SourceText))
	props.Set("signature", rt.NewStringObject(string(stub.Sig.Token())))
	props.Set("language", rt.NewStringObject(lambda.SourceLanguage))
	props.Set("environment", envObj)
	closure := e.alloc.New(intrin.Tree(), intrin.Signature().Return, props, nil)

	return rt.NewReturnUnwind(closure)
}

func arrayGetLength(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsArray() {
		return e.NewException(intrin.Tree(), "Expected receiver to be an array value")
	}
	arr := this.ArrayValue()
	if arr == nil {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null array value")
	}
	return rt.NewReturnUnwind(e.alloc.NewNumber(intrin.Tree(), float64(len(*arr))))
}

func arraySetLength(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsArray() {
		return e.NewException(intrin.Tree(), "Expected receiver to be an array value")
	}
	arr := this.ArrayValue()
	if arr == nil {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null array value")
	}
	if len(args) < 1 {
	}
	lengthFloat, ok := args[0].TryNumberValue()
	if !ok {
		return e.NewException(intrin.Tree(), "Expected length argument to be a number value")
	}
	length := int(lengthFloat)
	if length < 0 {
		return e.NewException(intrin.Tree(), "Expected length argument to be a positive number value")
	}

	// Update the size of the array to the requested length
	newArr := make([]*rt.Pointer, length)
	copy(*arr, newArr)
	*arr = newArr

	return rt.NewReturnUnwind(nil)
}

func stringGetLength(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsString() {
		return e.NewException(intrin.Tree(), "Expected receiver to be an string value")
	}
	str := this.StringValue()

	return rt.NewReturnUnwind(e.alloc.NewNumber(intrin.Tree(), float64(len(str))))
}

func stringToLowerCase(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsString() {
		return e.NewException(intrin.Tree(), "Expected receiver to be a string value")
	}
	str := this.StringValue()
	out := strings.ToLower(str)

	return rt.NewReturnUnwind(e.alloc.NewString(intrin.Tree(), out))
}

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
		contract.Assertf(propValuePointer.Getter() == nil, "Unexpected getter during serialization")
		propValue := propValuePointer.Obj()
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
		contract.Assertf(propValuePointer.Getter() == nil, "Unexpected getter during serialization")
		propValue := propValuePointer.Obj()
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
