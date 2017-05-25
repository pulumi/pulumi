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
	"fmt"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/eval/rt"
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

	// TODO: We are using the full environment available at execution time here, we should
	// instead capture only the free variables referenced in the function itself.
	envPropMap := rt.NewPropertyMap()
	for key, val := range stub.Env.Slots() {
		envPropMap.Set(rt.PropertyKey(key.Name()), val.Obj())
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
