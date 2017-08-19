// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package eval

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"bytes"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/binder"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/symbols"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/types"
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/eval/rt"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

func isFunction(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil)    // module function
	contract.Assert(len(args) == 1) // just one arg: the object to inquire about functionness
	_, isfunc := args[0].Type().(*symbols.FunctionType)
	ret := rt.Bools[isfunc]
	return rt.NewReturnUnwind(ret)
}

func defaultIfComputed(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	contract.Assert(this == nil) // module function.
	if len(args) <= 0 {
		return e.NewException(intrin.Tree(), "Missing 'obj' argument")
	} else if len(args) <= 1 {
		return e.NewException(intrin.Tree(), "Missing 'def' argument")
	}

	// If the object is computed, return def; otherwise, just return obj.
	obj := args[0]
	if obj.IsComputed() {
		return rt.NewReturnUnwind(args[1])
	}
	return rt.NewReturnUnwind(obj)
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

func objectKeys(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	var arr []*rt.Pointer
	if len(args) >= 1 && args[0] != nil {
		o := args[0]
		names := o.Properties().Stable()
		for _, name := range names {
			namePtr := rt.NewPointer(rt.NewStringObject(string(name)), true, nil, nil)
			arr = append(arr, namePtr)
		}
	}
	arrObj := rt.NewArrayObject(types.String, &arr)
	return rt.NewReturnUnwind(arrObj)
}

func printf(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	var message *rt.Object
	if len(args) >= 1 {
		message = args[0]
	} else {
		message = rt.Null
	}
	// TODO[pulumi/pulumi-fabric#169]: Move this to use a proper ToString() conversion.
	fmt.Print(message.String())
	return rt.NewReturnUnwind(nil)
}

func toString(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	var obj *rt.Object
	if len(args) >= 1 {
		obj = args[0]
	} else {
		obj = rt.Null
	}
	// TODO[pulumi/pulumi-fabric#169]: Move this to use a proper ToString() conversion.
	s := rt.NewStringObject(obj.String())
	return rt.NewReturnUnwind(s)
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
	if _, err := hasher.Write(byts); err != nil {
		return e.NewException(intrin.Tree(), "Failed to write out SHA1 bytes: %v", err)
	}
	sum := hasher.Sum(nil)
	hash := hex.EncodeToString(sum)

	hashObj := rt.NewStringObject(hash)
	return rt.NewReturnUnwind(hashObj)
}

type closureSerializer struct {
	node          diag.Diagable
	e             *evaluator
	envEntryCache map[*rt.Object]*rt.Object
}

func (s *closureSerializer) envEntryObjFor(obj *rt.Object) *rt.Object {
	envEntry, ok := s.envEntryCache[obj]
	if !ok {
		props := rt.NewPropertyMap()
		envEntry = rt.NewObject(types.Dynamic, nil, props, nil)
		s.envEntryCache[obj] = envEntry
		if obj.IsFunction() {
			// Serialize functions using serializeClosure
			stub := obj.FunctionValue()
			lambda, ok := stub.Func.(*ast.LambdaExpression)
			if ok {
				props.Set("closure", s.serializeClosure(stub, lambda))
			}
		} else if obj.IsBool() || obj.IsString() || obj.IsNumber() || obj.IsNull() {
			// Else if it's a primitive, pass through the object to serialize
			props.Set("json", obj)
		} else if obj.IsArray() {
			arr := *obj.ArrayValue()
			newArrElements := make([]*rt.Pointer, len(arr))
			for i, elem := range arr {
				newValue := s.envEntryObjFor(elem.Obj())
				newArrElements[i] = rt.NewPointer(newValue, false, nil, nil)
			}
			props.Set("arr", rt.NewArrayObject(types.Dynamic, &newArrElements))
		} else {
			// Else it's an object, and we recursively serialize it's properties.
			newObjProps := rt.NewPropertyMap()
			ownProps := obj.PropertyValues().Stable()
			for _, propKey := range ownProps {
				propPointer := obj.GetPropertyAddr(propKey, false, true)
				propObj := propPointer.Obj()
				newValue := s.envEntryObjFor(propObj)
				newObjProps.Set(propKey, newValue)
			}
			props.Set("obj", rt.NewObject(types.Dynamic, nil, newObjProps, nil))
		}
	}
	return envEntry
}

func (s *closureSerializer) serializeClosure(stub rt.FuncStub, lambda *ast.LambdaExpression) *rt.Object {
	envPropMap := rt.NewPropertyMap()
	for _, tok := range binder.FreeVars(stub.Func) {
		var name tokens.Name
		contract.Assertf(tok.Simple() || (tok.HasModule() && tok.HasModuleMember() && !tok.HasClassMember()),
			"Expected free variable to be simple name or reference to top-level module name")
		if tok.Simple() {
			name = tok.Name()
		} else {
			name = tokens.Name(tok.ModuleMember().Name())
		}
		pv := getDynamicNameAddrCore(stub.Env, stub.Module, name)
		if pv != nil {
			o := pv.Obj()
			entry := s.envEntryObjFor(o)
			envPropMap.Set(rt.PropertyKey(name), entry)
		}
		// Else the variable was not found, so we skip serializing it.
		// This will be true for references to globals which are not known to Lumi but
		// will be available within the runtime environment.
	}

	// Build up the properties for the returned Closure object
	props := rt.NewPropertyMap()
	props.Set("code", rt.NewStringObject(lambda.SourceText))
	props.Set("signature", rt.NewStringObject(string(stub.Sig.Token())))
	props.Set("language", rt.NewStringObject(lambda.SourceLanguage))
	props.Set("environment", rt.NewObject(types.Dynamic, nil, envPropMap, nil))
	return rt.NewObject(types.Dynamic, nil, props, nil)
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
	closureSerializer := &closureSerializer{
		node:          intrin.Tree(),
		e:             e,
		envEntryCache: map[*rt.Object]*rt.Object{},
	}
	closure := closureSerializer.serializeClosure(stub, lambda)
	return rt.NewReturnUnwind(closure)
}

func checkGetArray(intrin *rt.Intrinsic, e *evaluator, this *rt.Object) (*[]*rt.Pointer, *rt.Unwind) {
	if this == nil || this.IsNull() {
		return nil, e.NewNullObjectException(intrin.Tree())
	} else if !this.IsArray() {
		return nil, e.NewException(intrin.Tree(), "Expected receiver to be an array value")
	}
	arr := this.ArrayValue()
	if arr == nil {
		return nil, e.NewNullObjectException(intrin.Tree())
	}
	return arr, nil
}

func arrayGetLength(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	arr, uw := checkGetArray(intrin, e, this)
	if uw != nil {
		return uw
	}
	return rt.NewReturnUnwind(rt.NewNumberObject(float64(len(*arr))))
}

func arraySetLength(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	arr, uw := checkGetArray(intrin, e, this)
	if uw != nil {
		return uw
	}

	// Get and convert the 1st argument to a number.
	lengthFloat, ok := args[0].TryNumberValue()
	if !ok {
		return e.NewException(intrin.Tree(), "Expected length argument to be a number value")
	}
	length := int(lengthFloat)
	if length < 0 {
		return e.NewException(intrin.Tree(), "Expected length argument to be a positive number value")
	}

	// Update the size of the array to the requested length.
	newArr := make([]*rt.Pointer, length)
	copy(*arr, newArr)
	*arr = newArr

	return rt.NewReturnUnwind(nil)
}

// arrayPush implements Array.prototype.push.  It adds one or more elements to the end of an array and returns the new
// length of the array.  Please see the following link for details:
// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/push.
func arrayPush(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	arr, uw := checkGetArray(intrin, e, this)
	if uw != nil {
		return uw
	}
	for _, arg := range args {
		ptr := rt.NewPointer(arg, false, nil, nil)
		*arr = append(*arr, ptr)
	}
	return rt.NewReturnUnwind(rt.NewNumberObject(float64(len(*arr))))
}

// arrayPop implements Array.prototype.pop.  It removes the last element from an array and returns that element.
// This method changes the length of the array.  Please see the following link for details:
// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/pop.
func arrayPop(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	arr, uw := checkGetArray(intrin, e, this)
	if uw != nil {
		return uw
	}
	var ret *rt.Object
	if al := len(*arr); al > 0 {
		ret = ((*arr)[al-1]).Obj()
		*arr = (*arr)[:al-1]
	} else {
		ret = rt.Null
	}
	return rt.NewReturnUnwind(ret)
}

func stringGetLength(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil || this.IsNull() {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsString() {
		return e.NewException(intrin.Tree(), "Expected receiver to be an string value")
	}
	str := this.StringValue()

	return rt.NewReturnUnwind(rt.NewNumberObject(float64(len(str))))
}

func stringToLowerCase(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil || this.IsNull() {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsString() {
		return e.NewException(intrin.Tree(), "Expected receiver to be a string value")
	}
	str := this.StringValue()
	out := strings.ToLower(str)

	return rt.NewReturnUnwind(rt.NewStringObject(out))
}

func stringToUpperCase(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil || this.IsNull() {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsString() {
		return e.NewException(intrin.Tree(), "Expected receiver to be a string value")
	}
	str := this.StringValue()
	out := strings.ToUpper(str)

	return rt.NewReturnUnwind(rt.NewStringObject(out))
}

func stringSplit(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	if this == nil || this.IsNull() {
		return e.NewException(intrin.Tree(), "Expected receiver to be non-null")
	}
	if !this.IsString() {
		return e.NewException(intrin.Tree(), "Expected receiver to be a string value")
	}
	str := this.StringValue()
	arr := []*rt.Pointer{}
	if len(args) < 1 {
		arr = append(arr, rt.NewPointer(rt.NewStringObject(str), false, nil, nil))
	} else {
		sep := args[0]
		if !sep.IsString() {
			return e.NewException(intrin.Tree(), "Expected separate to be a string value")
		}
		parts := strings.Split(str, sep.StringValue())
		for _, part := range parts {
			arr = append(arr, rt.NewPointer(rt.NewStringObject(part), false, nil, nil))
		}
	}

	return rt.NewReturnUnwind(rt.NewArrayObject(types.String, &arr))
}

type jsonSerializer struct {
	stack  map[*rt.Object]bool
	intrin *rt.Intrinsic
	e      *evaluator
}

// See https://tc39.github.io/ecma262/2017/#sec-serializejsonproperty
func (s jsonSerializer) serializeJSONProperty(o *rt.Object) (string, *rt.Unwind) {
	if o == nil || o.IsNull() {
		return "null", nil
	} else if o.IsBool() {
		if o.BoolValue() {
			return "true", nil
		}
		return "false", nil
	} else if o.IsString() {
		return s.quote(o.StringValue()), nil
	} else if o.IsNumber() {
		return o.String(), nil
	} else if o.IsArray() {
		return s.serializeJSONArray(o)
	}
	return s.serializeJSONObject(o)
}

// See https://tc39.github.io/ecma262/2017/#sec-quotejsonstring
func (s jsonSerializer) quote(str string) string {
	escapes := map[rune]string{'\b': "\\b", '\f': "\\f", '\n': "\\n", '\r': "\\r", '\t': "\\t"}
	var buffer bytes.Buffer
	buffer.WriteRune('"')
	for _, c := range str {
		switch c {
		case '"', '\\':
			buffer.WriteRune('\\')
			buffer.WriteRune(c)
		case '\b', '\f', '\n', '\r', '\t':
			buffer.WriteString(escapes[c])
		default:
			if c < ' ' {
				buffer.WriteString(fmt.Sprintf("\\u%.4x", c))
			} else {
				buffer.WriteRune(c)
			}
		}
	}
	buffer.WriteRune('"')
	return buffer.String()
}

// See https://tc39.github.io/ecma262/2017/#sec-serializejsonobject
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

// See https://tc39.github.io/ecma262/2017/#sec-serializejsonarray
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
		return rt.NewReturnUnwind(rt.NewStringObject("{}"))
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
	return rt.NewReturnUnwind(rt.NewStringObject(str))
}

func jsonParse(intrin *rt.Intrinsic, e *evaluator, this *rt.Object, args []*rt.Object) *rt.Unwind {
	return e.NewException(intrin.Tree(), "Not yet implemented - jsonParse")
}
