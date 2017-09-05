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
    return Undefined(isolate);
}

Local<Value> SerializeFunction(Isolate *isolate, Local<Function> func, Local<Function> freevarsFunc);

// SerializeClosureEnvEntry serializes a JavaScript object as JSON so that it can be serialized and uploaded.
Local<Value> SerializeClosureEnvEntry(Isolate* isolate, Local<Value> v, Local<Function> freevarsFunc) {
    if (v.IsEmpty()) {
        // If the slot is empty, just return undefined.
        return Undefined(isolate);
    }

    Local<Object> entry = Object::New(isolate);
    if (v->IsUndefined() || v->IsNull() ||
            v->IsBoolean() || v->IsString() || v->IsNumber()) {
        // Serialize primitives as-is.
        entry->Set(String::NewFromUtf8(isolate, "json"), v);
    } else if (v->IsArray()) {
        // For arrays and objects, we must recursively serialize every element.
        Local<Array> arr = Local<Array>::Cast(v);
        Local<Array> newarr = Array::New(isolate, arr->Length());
        for (uint32_t i = 0; i < arr->Length(); i++) {
            Local<Integer> index = Integer::New(isolate, i);
            newarr->Set(index, SerializeClosureEnvEntry(isolate, arr->Get(index), freevarsFunc));
        }
        entry->Set(String::NewFromUtf8(isolate, "arr"), newarr);
    } else if (v->IsFunction()) {
        // Serialize functions recursively, and store them in a closure property.
        entry->Set(String::NewFromUtf8(isolate, "closure"),
                SerializeFunction(isolate, Local<Function>::Cast(v), freevarsFunc));
    } else if (v->IsObject()) {
        // For all other objects, recursively serialize all of its properties.
        Local<Object> obj = Local<Object>::Cast(v);
        Local<Object> newobj = Object::New(isolate);
        Local<Array> props = obj->GetPropertyNames(isolate->GetCurrentContext()).ToLocalChecked();
        for (uint32_t i = 0; i < props->Length(); i++) {
            Local<Value> propname = props->Get(Integer::New(isolate, i));
            newobj->Set(propname, SerializeClosureEnvEntry(isolate, obj->Get(propname), freevarsFunc));
        }
        entry->Set(String::NewFromUtf8(isolate, "obj"), newobj);
    } else {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Unsuported serialization closure entry")));
    }
    return entry;
}

Local<String> SerializeFunctionObjectText(Isolate *isolate, Local<Function> func) {
    // Serialize the code simply by calling toString on the Function.
    Local<Function> toString = Local<Function>::Cast(
            func->Get(String::NewFromUtf8(isolate, "toString")));
    Local<String> code = Local<String>::Cast(toString->Call(func, 0, nullptr));

    // Ensure that the code is a function expression (including arrows), and not a definition, etc.
    const char* badprefix = "[Function:";
    size_t badprefixLength = strlen(badprefix);
    if (code->Length() >= (int)badprefixLength) {
        char buf[badprefixLength];
        code->WriteUtf8(buf, badprefixLength);
        if (!strncmp(badprefix, buf, badprefixLength)) {
            isolate->ThrowException(Exception::TypeError(
                String::NewFromUtf8(isolate,
                    "Cannot serialize non-expression functions (such as definitions and generators)")));
        }
    }

    // Wrap the serialized function text in ()s so that it's a legal top-level script/module element.
    return String::Concat(
        String::Concat(String::NewFromUtf8(isolate, "("), code),
            String::NewFromUtf8(isolate, ")"));
}

// SerializeFunction serializes a JavaScript function expression and its associated closure environment.
Local<Value> SerializeFunction(Isolate *isolate, Local<Function> func, Local<Function> freevarsFunc) {
    // Get at the innards of the function.  Unfortunately, we need to use internal V8 APIs to do this,
    // as the closest public function, CreationContext, intentionally returns the non-closure Context for
    // Function objects (it returns the constructor context, which is not what we want).
    v8::internal::Handle<v8::internal::JSFunction> hackfunc(
            reinterpret_cast<v8::internal::JSFunction**>(const_cast<Function*>(*func)));
    v8::internal::Handle<v8::internal::Context> lexical(hackfunc->context());

    // Get the code as a string.
    Local<String> code = SerializeFunctionObjectText(isolate, func);

    // Compute the free variables by invoking the callback.
    std::vector<Local<String>> freevars;
    const unsigned freevarsArgc = 1;
    Local<Value> freevarsArgv[freevarsArgc] = { code };
    Local<Value> freevarsRet = freevarsFunc->Call(Null(isolate), freevarsArgc, freevarsArgv);
    if (freevarsRet.IsEmpty() || !freevarsRet->IsArray()) {
        isolate->ThrowException(Exception::TypeError(
            String::NewFromUtf8(isolate, "Free variables return expected to be an Array")));
    } else {
        Local<Array> freevarsArray = Local<Array>::Cast(freevarsRet);
        for (uint32_t i = 0; i < freevarsArray->Length(); i++) {
            Local<Integer> index = Integer::New(isolate, i);
            Local<Value> elem = freevarsArray->Get(index);
            if (elem.IsEmpty() || !elem->IsString()) {
                isolate->ThrowException(Exception::TypeError(
                    String::NewFromUtf8(isolate, "Free variable Array must contain only String elements")));
            } else {
                freevars.push_back(Local<String>::Cast(elem));
            }
        }
    }

    // Next, serialize all free variables as they exist in the function's original lexical environment.
    Local<Object> environment = Object::New(isolate);
    for (std::vector<Local<String>>::iterator it = freevars.begin(); it != freevars.end(); ++it) {
        // Look up the variable in the lexical closure of the function and then serialize it.
        Local<String> freevar = *it;
        Local<Value> v = Lookup(isolate, lexical, freevar);
        environment->Set(freevar, SerializeClosureEnvEntry(isolate, v, freevarsFunc));
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
    Local<Function> freevarsFunc = Local<Function>::Cast(args[1]);

    // Now go ahead and serialize it, and return the result.
    Local<Value> closure = SerializeFunction(isolate, func, freevarsFunc);
    args.GetReturnValue().Set(closure);
}

void init(Local<Object> exports) {
    NODE_SET_METHOD(exports, "serializeClosure", SerializeClosure);
}

NODE_MODULE(nativeruntime, init)

}  // namespace nativeruntime
