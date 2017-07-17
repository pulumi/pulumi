// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package eval

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Invoker implements an intrinsic function's functionality.
type Invoker func(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind

// Intrinsics contains the set of runtime functions that are callable by name through the Lumi standard library
// package.  Their functionality is implemented in the runtime because LumiIL cannot express the concepts they require
// to get their job done.  This includes things like dynamic introspection, invocation, and more.
var Intrinsics map[tokens.Token]Invoker

func init() {
	Intrinsics = map[tokens.Token]Invoker{
		// These intrinsics are exposed directly to users in the `lumi.runtime` package.
		"lumirt:index:defaultIfComputed": defaultIfComputed,
		"lumirt:index:isFunction":        isFunction,
		"lumirt:index:dynamicInvoke":     dynamicInvoke,
		"lumirt:index:objectKeys":        objectKeys,
		"lumirt:index:printf":            printf,
		"lumirt:index:sha1hash":          sha1hash,
		"lumirt:index:toString":          toString,
		"lumirt:index:jsonStringify":     jsonStringify,
		"lumirt:index:jsonParse":         jsonParse,
		"lumirt:index:serializeClosure":  serializeClosure,

		// These intrinsics are built-ins with no Lumi function exposed to users.
		// They are used as the implementation of core object APIs in the runtime.
		"lumi:builtin/array:getLength":    arrayGetLength,
		"lumi:builtin/array:setLength":    arraySetLength,
		"lumi:builtin/array:push":         arrayPush,
		"lumi:builtin/array:pop":          arrayPop,
		"lumi:builtin/string:getLength":   stringGetLength,
		"lumi:builtin/string:toLowerCase": stringToLowerCase,
		"lumi:builtin/string:toUpperCase": stringToUpperCase,
	}
}

func GetIntrinsicInvoker(intrinsic *rt.Intrinsic) Invoker {
	invoker, isintrinsic := Intrinsics[intrinsic.Token()]
	contract.Assert(isintrinsic)
	return invoker
}

// MaybeIntrinsic checks whether the given symbol is an intrinsic and, if so, swaps it out with the actual runtime
// implementation of that intrinsic.  If the symbol is not an intrinsic, the original symbol is simply returned.
func MaybeIntrinsic(sym symbols.Symbol) symbols.Symbol {
	switch s := sym.(type) {
	case *rt.Intrinsic:
		// Already an intrinsic; do not swap it out.
	case symbols.Function:
		// If this is a function whose token we recognize, create a new intrinsic symbol.  Note that we do not currently
		// cache these symbols because of the need to associate the AST node with the resulting symbol.
		tok := s.Token()
		if _, isintrinsic := Intrinsics[tok]; isintrinsic {
			sym = rt.NewIntrinsic(s)
		}
	}
	return sym
}
