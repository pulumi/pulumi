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

import * as lumi from "@lumi/lumi";
import {assert} from "./assert";
import {Boolean, Number, String, TypeError} from "../lib";

// The abstract operation ToString converts argument to a value of type String according to the table in
// https://tc39.github.io/ecma262/#sec-tostring.
export function toString(argument: Object): string {
    if (argument === undefined) {
        return "undefined";
    }
    if (argument === null) {
        return "null";
    }
    if (isBoolean(argument)) {
        if (argument === true) {
            return "true";
        }
        return "false";
    }
    if (isNumber(argument)) {
        // TODO: implement number formatting.
        return "NaN";
    }
    if (isSymbol(argument)) {
        throw new TypeError("toString invalid on symbols");
    }
    if (isString(argument)) {
        return <string>argument;
    }

    // For other objects, convert to a primitive value and stringify that.
    let primValue: Object = toPrimitive(argument, "string");
    return toString(primValue);
}

export function isObject(input: Object): boolean {
    return !isPrimitive(input);
}

export function isPrimitive(input: Object): boolean {
    return input === undefined || input === null ||
            isBoolean(input) || isNumber(input) || isString(input) || isSymbol(input)
}

export function isBoolean(input: Object): boolean {
    return (typeof(input) === "bool");
}

export function isNumber(input: Object): boolean {
    return (typeof(input) === "number");
}

export function isString(input: Object): boolean {
    return (typeof(input) === "string");
}

export function isSymbol(input: Object): boolean {
    // TODO: implement symbols.
    return false;
}

export function isFalsey(input: Object | undefined | null): boolean {
    // TODO: implement this based on the spec.
    if (input === false) {
        return true;
    }
    if (input === undefined) {
        return true;
    }
    if (input === null) {
        return true;
    }
    if (input === "") {
        return true;
    }
    return false;
}

// The abstract operation toPrimitive takes an input argument and an optional argument preferredType.  The abstract
// operation toPrimitive converts its input argument to a non-Object type.  If an object is capable of converting to
// more than one primitive type, it may use the optional hint preferredType to favor that type.  Conversion occurs
// according to the table in https://tc39.github.io/ecma262/#sec-toprimitive.
export function toPrimitive(input: Object, preferredType: string): Object {
    if (isPrimitive(input)) {
        return input;
    }

    let hint: string;
    if (isFalsey(preferredType)) {
        hint = "default";
    }
    else {
        hint = preferredType;
    }

    let exoticToPrim: Object | undefined = getMethod(input, "@@toPrimitive");
    if (!isFalsey(exoticToPrim)) {
        let result: Object = call(exoticToPrim!, input, [ hint ]);
        if (isObject(result)) {
            throw new TypeError("");
        }
        return result;
    }

    if (hint === "default") {
        hint = "number";
    }
    return ordinaryToPrimitive(input, hint);
}

// When the abstract operation OrdinaryToPrimitive is called with arguments O and hint, the steps outlined in
// https://tc39.github.io/ecma262/#sec-ordinarytoprimitive are taken.
export function ordinaryToPrimitive(o: Object, hint: string): Object {
    assert(isObject(o));

    let methodNames: string[];
    switch (hint) {
        case "string":
            methodNames = [ "toString", "valueOf" ];
            break;
        case "number":
            methodNames = [ "valueOf", "toString" ];
            break;
        default:
            assert(false);
            methodNames = [];
    }

    for (let i = 0; i < methodNames.length; i++) {
        let name: string = methodNames[i];
        let method: Object = get(o, name);
        if (isCallable(method)) {
            let result: Object = call(method, o, undefined);
            if (isPrimitive(result)) {
                return result;
            }
        }
    }

    throw new TypeError("invalid ordinaryToPrimitive type");
}

// The abstract operation ToObject converts argument to a value of type Object according to the table in
// https://tc39.github.io/ecma262/#sec-toobject.
export function toObject(argument: Object): Object {
    if (argument === undefined || argument === null) {
        throw new TypeError("toObject called on undefined or null value");
    }
    if (isBoolean(argument)) {
        return new Boolean(<boolean>argument);
    }
    if (isNumber(argument)) {
        return new Number(<number>argument);
    }
    if (isString(argument)) {
        return new String(<string>argument);
    }
    if (isSymbol(argument)) {
        // TODO: implement symbols.
    }
    return argument;
}

// The abstract operation Get is used to retrieve the value of a specific property of an object. The operation is called
// with arguments O and P where O is the object and P is the property key. This abstract operation performs the steps
// outlined in https://tc39.github.io/ecma262/#sec-get-o-p.
export function get(o: Object, p: string): Object {
    assert(isObject(o));
    assert(isPropertyKey(p));
    return (<any>o)[<any>p];
}

// The abstract operation GetV is used to retrieve the value of a specific property of an ECMAScript language value. If
// the value is not an object, the property lookup is performed using a wrapper object appropriate for the type of the
// value. The operation is called with arguments V and P where V is the value and P is the property key. This abstract
// operation performs the steps outlined in https://tc39.github.io/ecma262/#sec-getv.
export function getV(v: Object, p: Object): Object {
    assert(isPropertyKey(p));
    let o: Object = toObject(v);
    return (<any>o)[<any>p];
}

// The abstract operation GetMethod is used to get the value of a specific property of an ECMAScript language value when
// the value of the property is expected to be a function. The operation is called with arguments V and P where V is the
// ECMAScript language value, P is the property key. This abstract operation performs the steps outlined in
// https://tc39.github.io/ecma262/#sec-getmethod.
export function getMethod(v: Object, p: Object): Object | undefined {
    assert(isPropertyKey(p));
    let func: Object = getV(v, p);
    if (func === undefined || func === null) {
        return undefined;
    }
    if (!isCallable(func)) {
        throw new TypeError("expected a callable function");
    }
    return func;
}

// The abstract operation IsPropertyKey determines if argument, which must be an ECMAScript language value, is a value
// that may be used as a property key, as described in https://tc39.github.io/ecma262/#sec-ispropertykey.
export function isPropertyKey(argument: Object): boolean {
    if (isString(argument)) {
        return true;
    }
    if (isSymbol(argument)) {
        return true;
    }
    return false;
}

// The abstract operation Call is used to call the [[Call]] internal method of a function object. The operation is
// called with arguments F, V, and optionally argumentsList where F is the function object, V is an ECMAScript language
// value that is the this value of the [[Call]], and argumentsList is the value passed to the corresponding argument of
// the internal method. If argumentsList is not present, a new empty List is used as its value. This abstract operation
// performs the steps outlined in https://tc39.github.io/ecma262/#sec-call.
export function call(f: Object, v: Object, argumentsList?: Object[]): Object {
    if (!isCallable(f)) {
        throw new TypeError("function is not callable");
    }
    if (isFalsey(argumentsList)) {
        argumentsList = [];
    }
    return lumi.runtime.dynamicInvoke(f, v, argumentsList!);
}

// The abstract operation IsCallable determines if argument, which must be an ECMAScript language value, is a callable
// function with a [[Call]] internal method, as per https://tc39.github.io/ecma262/#sec-iscallable.
export function isCallable(argument: Object): boolean {
    if (!isObject(argument)) {
        return false;
    }
    return lumi.runtime.isFunction(argument);
}

