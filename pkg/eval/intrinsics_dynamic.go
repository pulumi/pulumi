// Copyright 2017 Pulumi, Inc. All rights reserved.

package eval

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func isFunction(intrin *Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil)    // module function
	contract.Assert(len(args) == 1) // just one arg: the object to inquire about functionness
	_, isfunc := args[0].Type().(*symbols.FunctionType)
	ret := e.alloc.NewBool(intrin.Node, isfunc)
	return rt.NewReturnUnwind(ret)
}

func dynamicInvoke(intrin *Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil)    // module function
	contract.Assert(len(args) == 3) // three args: obj, thisArg, and args

	// First ensure the target is a function.
	t := args[0].Type()
	if _, isfunc := t.(*symbols.FunctionType); !isfunc {
		return e.NewIllegalInvokeTargetException(intrin.Node, t)
	}

	// Fetch the function stub information (ignoring `this`, since we will pass our own).
	stub := args[0].FunctionValue()

	// Next, simply call through to the evalCall function, which will do all additional verification.
	stub.This = this // adjust this before the call (note this doesn't mutate the source stub; it's by-value).
	obj, uw := e.evalCallFuncStub(intrin.Node, stub, args...)
	if uw != nil {
		return uw
	}
	return rt.NewReturnUnwind(obj)
}
