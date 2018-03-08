// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

#include <cstring>
#include <vector>
#include <node.h>
#include <src/api.h>      // v8 internal APIs
#include <src/objects.h>  // v8 internal APIs
#include <src/contexts.h> // v8 internal APIs

#if NODE_MAJOR_VERSION != 6 || NODE_MINOR_VERSION != 10
#error "The Pulumi Fabric SDK only supports Node.js 6.10.x at the moment"
#endif

namespace nativeruntime {

using v8::Array;
using v8::Boolean;
using v8::Context;
using v8::Exception;
using v8::Function;
using v8::FunctionCallbackInfo;
using v8::Integer;
using v8::Isolate;
using v8::Local;
using v8::MaybeLocal;
using v8::Null;
using v8::Number;
using v8::Object;
using v8::Script;
using v8::String;
using v8::Value;

// Lookup restores a context and looks up a variable name inside of it.
Local<Value> Lookup(
        Isolate* isolate, v8::internal::Handle<v8::internal::Context> context,
        Local<String> name, bool throwOnFailure) {
    // First perform the lookup in the current chain.  This unfortunately requires accessing internal
    // V8 APIs so that we can inspect the chain with the necessary flags and resulting objects.
    int index;
    v8::internal::PropertyAttributes attributes;
    v8::internal::BindingFlags bflags;
    v8::internal::Handle<v8::internal::String> hackname(
            reinterpret_cast<v8::internal::String**>(const_cast<String*>(*name)));
    v8::internal::Handle<v8::internal::Object> lookup =
        context->Lookup(
                hackname,
                v8::internal::ContextLookupFlags::FOLLOW_CHAINS,
                &index, &attributes, &bflags);

    // Now check the result.  There are several legal possibilities.
    if (!lookup.is_null()) {
        if (lookup->IsContext()) {
            // The result was found in a context; index contains the slot number within that context.
            v8::internal::Isolate* hackiso =
                    reinterpret_cast<v8::internal::Isolate*>(isolate);
            return v8::Utils::Convert<v8::internal::Object, Object>(
                    v8::internal::FixedArray::get(v8::internal::Context::cast(*lookup), index, hackiso));
        } else if (lookup->IsJSObject()) {
            // The result was a named property inside of a context extension (such as eval); we can return it as-is.
            return v8::Utils::Convert<v8::internal::Object, Object>(lookup);
        }
    }

    if (throwOnFailure) {
        // If we fell through, either the lookup is null, or the object wasn't of the expected type.
        // In either case, this is an error (possibly a bug), and we will throw and return undefined
        // so we can keep going.
        char namestr[255];
        name->WriteUtf8(namestr, 255);
        Local<String> errormsg = String::Concat(
            String::NewFromUtf8(isolate, "Unexpected missing variable in closure environment: "),
                String::NewFromUtf8(isolate, namestr));
        isolate->ThrowException(Exception::Error(errormsg));
    }

    return Local<Value>();
}

// ComputeAndSerializeCapturedFreeVariables serializes a function and its closure environment into a form that is amenable to persistence
// as simple JSON.  Like toString, it includes the full text of the function's source code, suitable for execution.
// Unlike toString, it actually includes information about the captured environment.
void LookupCapturedVariableValue(const FunctionCallbackInfo<Value>& args) {
    Isolate* isolate = args.GetIsolate();

    // Ensure the first argument is a proper function expression object.
    if (args.Length() < 1 || args[0]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required function argument (arg-0)")));
        return;
    } else if (!args[0]->IsFunction()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "User function (arg-0) must be a Function object")));
        return;
    }

    if (args.Length() < 2 || args[1]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required string argument (arg-1)")));
        return;
    } else if (!args[1]->IsString()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Function code argument (arg-1) must be string")));
        return;
    }

    if (args.Length() < 3 || args[2]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required bool argument (arg-2)")));
        return;
    } else if (!args[2]->IsBoolean()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Function code argument (arg-2) must be boolean")));
        return;
    }

    auto func = Local<Function>::Cast(args[0]);
    auto freeVariable = Local<String>::Cast(args[1]);
    auto throwOnFailure = Local<Boolean>::Cast(args[2]);

    // Get at the innards of the function.  Unfortunately, we need to use internal V8 APIs to do this,
    // as the closest public function, CreationContext, intentionally returns the non-closure Context for
    // Function objects (it returns the constructor context, which is not what we want).
    v8::internal::Handle<v8::internal::JSFunction> hackfunc(
            reinterpret_cast<v8::internal::JSFunction**>(const_cast<Function*>(*func)));
    v8::internal::Handle<v8::internal::Context> lexical(hackfunc->context());

    Local<Value> v = Lookup(isolate, lexical, freeVariable, throwOnFailure->Value());

    args.GetReturnValue().Set(v);
}

void GetFunctionFile(const FunctionCallbackInfo<Value>& args) {
    auto func = Local<Function>::Cast(args[0]);
    auto origin = func->GetScriptOrigin();

    args.GetReturnValue().Set(origin.ResourceName());
}

void GetFunctionLine(const FunctionCallbackInfo<Value>& args) {
    Isolate* isolate = args.GetIsolate();

    auto func = Local<Function>::Cast(args[0]);
    args.GetReturnValue().Set(Integer::New(isolate, func->GetScriptLineNumber()));
}

void init(Local<Object> exports) {
    NODE_SET_METHOD(exports, "lookupCapturedVariableValue", LookupCapturedVariableValue);
    NODE_SET_METHOD(exports, "getFunctionFile", GetFunctionFile);
    NODE_SET_METHOD(exports, "getFunctionLine", GetFunctionLine);
}

NODE_MODULE(nativeruntime, init)

}  // namespace nativeruntime
