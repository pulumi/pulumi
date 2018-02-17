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
using v8::Context;
using v8::Exception;
using v8::Function;
using v8::FunctionCallbackInfo;
using v8::Integer;
using v8::Isolate;
using v8::Local;
using v8::MaybeLocal;
using v8::Null;
using v8::Object;
using v8::Script;
using v8::String;
using v8::Value;

// CreateClosure allocates a new closure object which matches the definition on the JavaScript side.
Local<Object> CreateClosure(Isolate* isolate, Local<String> code, Local<Object> environment) {
    Local<Object> closure = Object::New(isolate);
    closure->Set(String::NewFromUtf8(isolate, "code"), code);
    closure->Set(String::NewFromUtf8(isolate, "runtime"), String::NewFromUtf8(isolate, "nodejs"));
    closure->Set(String::NewFromUtf8(isolate, "environment"), environment);
    return closure;
}

// Lookup restores a context and looks up a variable name inside of it.
Local<Value> Lookup(Isolate* isolate, v8::internal::Handle<v8::internal::Context> context, Local<String> name) {
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

    // If we fell through, either the lookup is null, or the object wasn't of the expected type.  In either case,
    // this is an error (possibly a bug), and we will throw and return undefined so we can keep going.
    char namestr[255];
    name->WriteUtf8(namestr, 255);
    Local<String> errormsg = String::Concat(
        String::NewFromUtf8(isolate, "Unexpected missing variable in closure environment: "),
            String::NewFromUtf8(isolate, namestr));
    isolate->ThrowException(Exception::Error(errormsg));
    return Local<Value>();
}

Local<String> SerializeFunctionCode(Isolate *isolate, Local<Function> func) {
    // Serialize the code simply by calling toString on the Function.
    auto toString = Local<Function>::Cast(func->Get(String::NewFromUtf8(isolate, "toString")));
    auto v8CodeString = Local<String>::Cast(toString->Call(func, 0, nullptr));

    v8::String::Utf8Value utf8CodeString(v8CodeString->ToString());
    std::string code = std::string(*utf8CodeString);

    std::string badPrefix("[Function:");
    if (code.compare(0, badPrefix.length(), badPrefix) == 0) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate,
                "Cannot serialize non-expression functions (such as definitions and generators)")));
    }

    // Ensure that the code is a function expression (including arrows), and not a definition, etc.

    std::string openParen("(");
    std::string funcString("function");

    if (code.compare(0, openParen.length(), openParen) == 0 ||
        code.compare(0, funcString.length(), funcString) == 0) {

        // lambda or simple function expression.  i.e. '() => { ... }' or 'function () { }'
        // wrap with parens to make into an expression that can be parsed at the top level
        // of a JS file.
        code = "(" + code + ")";
    } else {
        // We got a method here.  Which v8 represents like 'foo() { }'.  So we wrap as
        // '(functoin foo() { })' so it can be parsed at the top level of a JS file.
        code = "(function " + code + ")";
    }

    return String::NewFromUtf8(isolate, code.c_str());
}

// SerializeFunction serializes a JavaScript function expression and its associated closure environment.
Local<Value> SerializeFunction(Isolate *isolate, Local<Function> func,
        Local<Function> freeVarsFunc, Local<Function> envEntryFunc, Local<Object> envEntryCache) {
    // Get at the innards of the function.  Unfortunately, we need to use internal V8 APIs to do this,
    // as the closest public function, CreationContext, intentionally returns the non-closure Context for
    // Function objects (it returns the constructor context, which is not what we want).
    v8::internal::Handle<v8::internal::JSFunction> hackfunc(
            reinterpret_cast<v8::internal::JSFunction**>(const_cast<Function*>(*func)));
    v8::internal::Handle<v8::internal::Context> lexical(hackfunc->context());

    // Get the code as a string.
    Local<String> code = SerializeFunctionCode(isolate, func);

    // Compute the free variables by invoking the callback.
    const unsigned freeVarsArgc = 1;
    Local<Value> freeVarsArgv[freeVarsArgc] = { code };
    Local<Value> freeVarsRet = freeVarsFunc->Call(Null(isolate), freeVarsArgc, freeVarsArgv);
    if (freeVarsRet.IsEmpty()) {
        // Only empty if the function threw an exception.  Return early to propagate it.
        return Local<Value>();
    } else if (!freeVarsRet->IsArray()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Free variables return expected to be an Array")));
        return Local<Value>();
    }

    // Now check all elements and produce a vector we can use below.
    // std::vector<Local<String>> freeVars;
    // Local<Array> freeVarsArray = Local<Array>::Cast(freeVarsRet);
    // for (uint32_t i = 0; i < freeVarsArray->Length(); i++) {
    //     Local<Integer> index = Integer::New(isolate, i);
    //     Local<Value> elem = freeVarsArray->Get(index);
    //     if (elem.IsEmpty() || !elem->IsString()) {
    //         isolate->ThrowException(Exception::TypeError(
    //             String::NewFromUtf8(isolate, "Free variable Array must contain only String elements")));
    //         return Local<Value>();
    //     }
    //     freeVars.push_back(Local<String>::Cast(elem));
    // }

    auto zero = Integer::New(isolate, 0);
    auto one = Integer::New(isolate, 1);

    Local<Object> environment = Object::New(isolate);

    // Next, serialize all free variables as they exist in the function's original lexical environment.
    auto freeVarsArray = Local<Array>::Cast(freeVarsRet);
    for (uint32_t i = 0; i < freeVarsArray->Length(); i++) {
        Local<Integer> index = Integer::New(isolate, i);
        Local<Array> elemAndProps = Local<Array>::Cast(freeVarsArray->Get(index));

        Local<String> freevar = Local<String>::Cast(elemAndProps->Get(zero));
        Local<Value> props = elemAndProps->Get(one);

        // Look up the variable in the lexical closure of the function and then serialize it.
        Local<Value> v = Lookup(isolate, lexical, freevar);
        if (v.IsEmpty()) {
            // Only empty if an error was thrown; bail eagerly to propagate it.
            return Local<Value>();
        }
        const unsigned envEntryArgc = 3;
        Local<Value> envEntryArgv[envEntryArgc] = { v, props, envEntryCache };
        Local<Value> envEntry = envEntryFunc->Call(Null(isolate), envEntryArgc, envEntryArgv);
        if (envEntry.IsEmpty()) {
            return Local<Value>();
        }
        environment->Set(freevar, envEntry);
    }

    // Finally, produce a closure object with all the appropriate records, and return it.
    return CreateClosure(isolate, code, environment);
}

// serializeClosure serializes a function and its closure environment into a form that is amenable to persistence
// as simple JSON.  Like toString, it includes the full text of the function's source code, suitable for execution.
// Unlike toString, it actually includes information about the captured environment.
void SerializeClosure(const FunctionCallbackInfo<Value>& args) {
    Isolate* isolate = args.GetIsolate();

    // Ensure the first argument is a proper function expression object.
    if (args.Length() < 1 || args[0]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required function argument")));
        return;
    } else if (!args[0]->IsFunction()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Function argument must be a Function object")));
        return;
    }
    Local<Function> func = Local<Function>::Cast(args[0]);

    // And that the second is a callback we can use to compute the free variables list.
    if (args.Length() < 2 || args[1]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required free variables calculator")));
        return;
    } else if (!args[1]->IsFunction()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Free variables argument must be a Function object")));
        return;
    }
    Local<Function> freeVarsFunc = Local<Function>::Cast(args[1]);

    // And that the third is a callback we can use to serialize environment entries.
    if (args.Length() < 3 || args[2]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required env-entry serializer function")));
        return;
    } else if (!args[2]->IsFunction()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Env-entry serializer argument must be a Function object")));
        return;
    }
    Local<Function> envEntryFunc = Local<Function>::Cast(args[2]);

    // And that the fourth is an entry cache used to backstop mutually recursive captures.
    if (args.Length() < 4 || args[3]->IsUndefined()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Missing required env-entry cache")));
        return;
    }
    Local<Object> envEntryCache = Local<Object>::Cast(args[3]);

    // Now go ahead and serialize it, and return the result.
    Local<Value> closure = SerializeFunction(isolate, func, freeVarsFunc, envEntryFunc, envEntryCache);
    if (!closure.IsEmpty()) {
        args.GetReturnValue().Set(closure);
    }
}

void init(Local<Object> exports) {
    NODE_SET_METHOD(exports, "serializeClosure", SerializeClosure);
}

NODE_MODULE(nativeruntime, init)

}  // namespace nativeruntime
