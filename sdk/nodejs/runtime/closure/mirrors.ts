// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as v8 from "./v8";

import * as mirrorsPostV11 from "./mirrorsPostV11";
import * as mirrorsPreV11 from "./mirrorsPreV11";

type RemoteObjectId = string;
type UnserializableValue = string;

export interface Mirror {
    /** Object type. */
    type: "function" | "object" | "number" | "string" | "undefined" | "boolean" | "symbol";
    /** Object subtype hint. Specified for `object` type values only. */
    subtype?: string;
    /** Object class (constructor) name. Specified for `object` type values only. */
    // className?: string;
    /** Remote object value in case of primitive values or JSON values (if it was requested). */
    value?: any;
    /** Primitive value which can not be JSON-stringified does not have `value`, but gets this property. */
    unserializableValue?: UnserializableValue;
    /** Unique object identifier (for non-primitive values). */
    objectId?: RemoteObjectId;

    __isMirror: true;
}

export interface FunctionMirror extends Mirror {
    type: "function";
    objectId: RemoteObjectId;

    /** contains the result of calling '.toString()' on the function instance. */
    // description: string;

    // properties that never appear
    // className?: never;
    subtype?: never;
    value?: never;
    unserializableValue?: never;
}

export interface SymbolMirror extends Mirror {
    type: "symbol";
    objectId: RemoteObjectId;

    // properties that never appear
    subtype?: never;
    // className?: never;
    unserializableValue?: never;
    // description?: never;
    value?: never;
}

export interface StringMirror extends Mirror {
    type: "string";
    value: string;

    // properties that never appear
    subtype?: never;
    // className?: never;
    unserializableValue?: never;
    objectId?: never;
    // description?: never;
}

export interface NumberMirror extends Mirror {
    type: "number";

    // The numeric value, when that numeric value is representable in JSON
    value: number | undefined;

    // A string representation of the numeric value is not representable in JSON (for example
    // 'NaN').
    unserializableValue: string | undefined;

    // properties that never appear
    subtype?: never;
    // className?: never;
    objectId?: never;
    // description?: never;
}

export interface BooleanMirror extends Mirror {
    type: "boolean";
    value: boolean;

    // properties that never appear
    subtype?: never;
    // className?: never;
    unserializableValue?: never;
    objectId?: never;
    // description?: never;
}

export interface UndefinedMirror extends Mirror {
    type: "undefined";

    // properties that never appear
    value?: never;
    subtype?: never;
    // className?: never;
    unserializableValue?: never;
    objectId?: never;
    // description?: never;
}

export interface ObjectMirror extends Mirror {
    type: "object";

    // Some ObjectMirrors always have a subtype.
    subtype?: "null" | "regexp" | "promise" | "array";
}

export interface NullMirror extends ObjectMirror {
    subtype: "null";

    // NullMirror always has a null value.
    value: null;

    // properties that never appear
    // className?: never;
    unserializableValue?: never;
    objectId?: never;
    // description?: never;
}

export interface RegExpMirror extends ObjectMirror {
    subtype: "regexp";
    // className: "RegExp";
    objectId: RemoteObjectId;

    // properties that never appear
    value?: never;
    unserializableValue?: never;
    // description?: never;
}

export interface PromiseMirror extends ObjectMirror {
    subtype: "promise";
    // className: "Promise";
    objectId: RemoteObjectId;

    // properties that never appear
    value?: never;
    unserializableValue?: never;
    // description?: never;
}

export interface ArrayMirror extends ObjectMirror {
    subtype: "array";
    // className: "Array";
    objectId: RemoteObjectId;

    // properties that never appear
    value?: never;
    unserializableValue?: never;
    // description?: never;
}

export interface MirrorPropertyDescriptor {
    /** Property name or symbol description. Only one of [name] or [symbol] will be set. */
    name?: StringMirror;
    /** Property symbol object, if the property is of the `symbol` type.  Only one of [name] or [symbol] will be set. */
    symbol?: SymbolMirror;
    /** The value associated with the property. */
    value?: Mirror;
    /** True if the value associated with the property may be changed (data descriptors only). */
    writable?: boolean;
    /**
     * A function which serves as a getter for the property, or `undefined` if there is no getter
     * (accessor descriptors only).
     */
    get?: FunctionMirror;
    /**
     * A function which serves as a setter for the property, or `undefined` if there is no setter
     * (accessor descriptors only).
     */
    set?: FunctionMirror;
    /**
     * True if the type of this property descriptor may be changed and if the property may be
     * deleted from the corresponding object.
     */
    configurable?: boolean;
    /**
     * True if this property shows up during enumeration of the properties on the corresponding
     * object.
     */
    enumerable?: boolean;
}

export interface FunctionDetails {
    /** Name of the function, if it has one. */
    name: string;

    /** Location of the function as best as can be determined. */
    location: { file: string, line: number, column: number };

    /** Code of the function.  Equivalent to calling .toString() on the original function instance. */
    code: string;
}

export function isMirror(val: any): val is Mirror {
    return val && val.__isMirror;
}

export function isUndefinedMirror(mirror: Mirror | undefined): mirror is UndefinedMirror {
    return isMirror(mirror) && mirror.type === "undefined";
}

export function isObjectMirror(mirror: Mirror | undefined): mirror is ObjectMirror {
    return isMirror(mirror) && mirror.type === "object";
}

export function isStringMirror(mirror: Mirror | undefined): mirror is StringMirror {
    return isMirror(mirror) && mirror.type === "string";
}

export function isBooleanMirror(mirror: Mirror | undefined): mirror is BooleanMirror {
    return isMirror(mirror) && mirror.type === "boolean";
}

export function isNumberMirror(mirror: Mirror | undefined): mirror is NumberMirror {
    return isMirror(mirror) && mirror.type === "number";
}

export function isFunctionMirror(mirror: Mirror | undefined): mirror is FunctionMirror {
    return isMirror(mirror) && mirror.type === "function";
}

export function isNullMirror(mirror: Mirror | undefined): mirror is NullMirror {
    return isObjectMirror(mirror) && mirror.subtype === "null";
}

export function isPromiseMirror(mirror: Mirror | undefined): mirror is PromiseMirror {
    return isObjectMirror(mirror) && mirror.subtype === "promise";
}

export function isArrayMirror(mirror: Mirror | undefined): mirror is ArrayMirror {
    return isObjectMirror(mirror) && mirror.subtype === "array";
}

export function isRegExpMirror(mirror: Mirror | undefined): mirror is RegExpMirror {
    return isObjectMirror(mirror) && mirror.subtype === "regexp";
}

export function isSymbolMirror(mirror: Mirror | undefined): mirror is SymbolMirror {
    return isMirror(mirror) && mirror.type === "symbol";
}

export function isTruthy(mirror: Mirror) {
    if (isUndefinedMirror(mirror)) {
        return false;
    }
    if (isNullMirror(mirror)) {
        return false;
    }
    if (isStringMirror(mirror)) {
        return mirror.value ? true : false;
    }
    if (isBooleanMirror(mirror)) {
        return mirror.value;
    }
    if (isNumberMirror(mirror)) {
        return mirror.value ? true : false;
    }
    if (isFunctionMirror(mirror)) {
        return true;
    }
    if (isSymbolMirror(mirror)) {
        return true;
    }

    // 'object' test handles things like regexp/array/promise/null.
    if (isObjectMirror(mirror)) {
        return true;
    }

    throw new Error("Unhandled isTruthy case: " + JSON.stringify(mirror));
}

export function isFalsy(mirror: Mirror) {
    return !isTruthy(mirror);
}

export function isUndefinedOrNullMirror(mirror: Mirror) {
    return isUndefinedMirror(mirror) || isNullMirror(mirror);
}

export function isStringValue(mirror: Mirror | undefined, val: string): boolean {
    return isStringMirror(mirror) && mirror.value === val;
}

export function getNameOrSymbol(descriptor: MirrorPropertyDescriptor): SymbolMirror | StringMirror {
    if (descriptor.symbol === undefined && descriptor.name === undefined) {
        throw new Error("Descriptor didn't have symbol or name: " + JSON.stringify(descriptor));
    }

    return descriptor.symbol || descriptor.name!!;
}

export type MirrorType<T> =
    T extends undefined ? UndefinedMirror :
    T extends null ? NullMirror :
    T extends string ? StringMirror :
    T extends number ? NumberMirror :
    T extends boolean ? BooleanMirror :
    T extends RegExp ? RegExpMirror :
    T extends symbol ? SymbolMirror :
    T extends Array<infer A> ? ArrayMirror :
    T extends Promise<infer B> ? PromiseMirror :
    T extends Function ? FunctionMirror : Mirror;

const mirrorModule = v8.isNodeAtLeastV11 ? mirrorsPostV11 : mirrorsPreV11;

/** Given a value, returns the Mirror for that value. */
export const getMirrorAsync = mirrorModule.getMirrorAsync;

/** Given a Mirror of some value V, returns the Mirror for Object.getPrototypeOf(V) */
export const getPrototypeOfMirrorAsync = mirrorModule.getPrototypeOfMirrorAsync;

/** Given a Mirror for some Promise P, returns the Mirror for `await P` */
export const getPromiseMirrorValueAsync = mirrorModule.getPromiseMirrorValueAsync;

/**
 * Given a Mirror for some value V, returns the Mirror property descriptors for
 * Object.getOwnPropertyDescriptors(V)
 */
export const getOwnPropertyDescriptorsAsync = mirrorModule.getOwnPropertyDescriptorsAsync;

/**
 * Given a Mirror for some value V and a property descriptor for some property P, returns the Mirror
 * for Object.getOwnProperty(V, P);
 */
export const getOwnPropertyAsync = mirrorModule.getOwnPropertyAsync;

export const lookupCapturedVariableAsync = mirrorModule.lookupCapturedVariableAsync;

export const callFunctionOn = mirrorModule.callFunctionOn;
export const callAccessorOn = mirrorModule.callAccessorOn;

export const getFunctionDetailsAsync = mirrorModule.getFunctionDetailsAsync;
