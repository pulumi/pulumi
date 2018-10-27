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

type RemoteObjectId = string;

type UnserializableValue = string;

export interface Mirror {
    /** Object type. */
    type: "function" | "object" | "number" | "string" | "undefined" | "boolean";
    /** Object subtype hint. Specified for `object` type values only. */
    subtype?: string;
    /** Object class (constructor) name. Specified for `object` type values only. */
    className?: string;
    /** Remote object value in case of primitive values or JSON values (if it was requested). */
    value?: any;
    /** Primitive value which can not be JSON-stringified does not have `value`, but gets this property. */
    unserializableValue?: UnserializableValue;
    /** Unique object identifier (for non-primitive values). */
    objectId?: RemoteObjectId;
}

export interface FunctionMirror extends Mirror {
    type: "function";
    className: "Function" | "Array";
    objectId: RemoteObjectId;

    /** contains the result of calling '.toString()' on the function instance. */
    description: string;

    // properties that never appear
    subtype?: never;
    value?: never;
    unserializableValue?: never;

    // Temporary Deviation from v8 to make transition easier.
    name: string;
    location: { file: string, line: number, column: number };
}

export interface StringMirror extends Mirror {
    type: "string";
    value: string;

    // properties that never appear
    subtype?: never;
    className?: never;
    unserializableValue?: never;
    objectId?: never;
    description?: never;
}

export interface NumberMirror extends Mirror {
    type: "number";

    // The numeric value, when that numeric value is representable in JSON
    value: number;

    // A string representation of the numeric value is not representable in JSON (for example
    // 'NaN').
    unserializableValue: string;

    // properties that never appear
    subtype?: never;
    className?: never;
    objectId?: never;
    description?: never;
}

export interface BooleanMirror extends Mirror {
    type: "boolean";
    value: boolean;

    // properties that never appear
    subtype?: never;
    className?: never;
    unserializableValue?: never;
    objectId?: never;
    description?: never;
}

export interface UndefinedMirror extends Mirror {
    type: "undefined";

    // properties that never appear
    value?: never;
    subtype?: never;
    className?: never;
    unserializableValue?: never;
    objectId?: never;
    description?: never;
}

export interface ObjectMirror extends Mirror {
    type: "object";

    // ObjectMirrors always have a subtype.
    subtype: "null" | "regexp" | "promise" | "array";
}

export interface NullMirror extends ObjectMirror {
    subtype: "null";

    // NullMirror always has a null value.
    value: null;

    // properties that never appear
    className?: never;
    unserializableValue?: never;
    objectId?: never;
    description?: never;
}

export interface RegExpMirror extends ObjectMirror {
    subtype: "regexp";
    className: "RegExp";
    objectId: string;

    // properties that never appear
    value?: null;
    unserializableValue?: never;
    description?: never;
}

export interface PromiseMirror extends ObjectMirror {
    subtype: "promise";
    className: "Promise";
    objectId: string;

    // properties that never appear
    value?: null;
    unserializableValue?: never;
    description?: never;
}

export interface ArrayMirror extends ObjectMirror {
    subtype: "array";
    className: "Array";
    objectId: string;

    // properties that never appear
    value?: null;
    unserializableValue?: never;
    description?: never;
}

type MirrorType<T> =
    T extends undefined ? UndefinedMirror :
    T extends null ? NullMirror :
    T extends string ? StringMirror :
    T extends number ? NumberMirror :
    T extends boolean ? BooleanMirror :
    T extends RegExp ? RegExpMirror :
    T extends Array<infer U> ? ArrayMirror :
    T extends Promise<infer U> ? PromiseMirror :
    T extends Function ? FunctionMirror : Mirror;

type ValueType<TMirror> =
    TMirror extends UndefinedMirror ? undefined :
    TMirror extends NullMirror ? null :
    TMirror extends StringMirror ? string :
    TMirror extends NumberMirror ? number :
    TMirror extends BooleanMirror ? boolean :
    TMirror extends RegExpMatchArray ? RegExp :
    TMirror extends ArrayMirror ? any[] :
    TMirror extends PromiseMirror ? Promise<any> :
    TMirror extends FunctionMirror ? Function : any;

let currentMirrorId = 0;
const functionIdToFunc = new Map<RemoteObjectId, Function>();

const valToMirror = new Map<any, Mirror>();
const mirrorToVal = new Map<Mirror, any>();

// Not for general use.  Only when transitioning over to the inspector API.
export function getValueForMirror<TMirror extends Mirror>(mirror: TMirror): ValueType<TMirror> {
    const val = mirrorToVal.get(mirror);
    if (!val) {
        throw new Error("Didn't have value for mirror: " + JSON.stringify(mirror));
    }

    return val;
}

export async function getMirrorAsync<T>(val: T): Promise<MirrorType<T>> {
    let mirror = valToMirror.get(val);
    if (mirror) {
        return <any>mirror;
    }

    mirror = await createMirrorAsync();
    valToMirror.set(val, mirror);
    mirrorToVal.set(mirror, val);
    return <any>mirror;

    async function createMirrorAsync(): Promise<Mirror> {
        const mirrorId = "id" + currentMirrorId++;

        if (typeof val === "string") {
            const stringMirror: StringMirror = {
                type: "string",
                value: val,
            };

            return stringMirror;
        }

        if (val instanceof Function) {
            const funcMirror: FunctionMirror = {
                type: "function",
                className: "Function",
                objectId: mirrorId,
                description: val.toString(),
                name: val.name,
                location: await v8.getFunctionLocationAsync(val),
            };

            functionIdToFunc.set(mirrorId, val);
            return funcMirror;
        }

        throw new Error("NYI: unhandled createMirrorAsync case.");
    }
}

export async function getPrototypeOfMirrorAsync(mirror: Mirror): Promise<Mirror> {
    const proto = Object.getPrototypeOf(getValueForMirror(mirror));
    return getMirrorAsync(proto);
}

export function callFunctionOn(mirror: Mirror, funcName: string, args: Mirror[] = []): Promise<Mirror> {
    if (!mirror.objectId) {
        throw new Error("Can't call function on mirror without an objectId: " + JSON.stringify(mirror));
    }

    let index = 0;
    for (const arg of args) {
        if (!arg.objectId) {
            throw new Error(`$args[${index} did not have objectId: ${JSON.stringify(arg)}`);
        }

        index++;
    }

    const realInstance = getValueForMirror(mirror);
    const realArgs = args.map(a => getValueForMirror(a));
    const func: Function = realInstance[funcName];

    if (!func) {
        throw new Error(`No function called ${funcName} found on mirror: ${JSON.stringify(mirror)}`);
    }

    if (!(func instanceof Function)) {
        throw new Error(`${funcName} was not a function: ${JSON.stringify(func)}`);
    }

    const res = func.call(realInstance, ...realArgs);
    return getMirrorAsync(res);

    // const resType = await new Promise<inspector.Runtime.CallFunctionOnReturnType>((resolve, reject) => {
    //     session.post("Runtime.callFunctionOn", {
    //         objectId: mirror.objectId,
    //         functionDeclaration: funcName,
    //         arguments: args.map(a => ({ objectId: a.objectId })),
    //     }, (err, res) => err ? reject(err) : resolve(res));
    // });

    // if (resType.exceptionDetails) {

    // }
}

export async function getMirrorMemberAsync(mirror: Mirror, memberName: string): Promise<Mirror> {
    if (isUndefinedOrNullMirror(mirror)) {
        throw new Error(`Trying to get member ${memberName} off null/undefined: ${JSON.stringify(mirror)}`);
    }

    const val = getValueForMirror(mirror);
    const member = val[memberName];
    return getMirrorAsync(member);
}

export function isUndefinedOrNullMirror(mirror: Mirror) {
    return isUndefinedMirror(mirror) || isNullMirror(mirror);
}

export function isUndefinedMirror(mirror: Mirror): mirror is UndefinedMirror {
    return mirror.type === "undefined";
}

export function isObjectMirror(mirror: Mirror): mirror is ObjectMirror {
    return mirror.type === "object";
}

export function isStringMirror(mirror: Mirror): mirror is StringMirror {
    return mirror.type === "string";
}

export function isBooleanMirror(mirror: Mirror): mirror is BooleanMirror {
    return mirror.type === "boolean";
}

export function isNumberMirror(mirror: Mirror): mirror is NumberMirror {
    return mirror.type === "number";
}

export function isFunctionMirror(mirror: Mirror): mirror is FunctionMirror {
    return mirror.type === "function";
}

export function isNullMirror(mirror: Mirror): mirror is NullMirror {
    return isObjectMirror(mirror) && mirror.subtype === "null";
}

export function isPromiseMirror(mirror: Mirror): mirror is PromiseMirror {
    return isObjectMirror(mirror) && mirror.subtype === "promise";
}

export function isArrayMirror(mirror: Mirror): mirror is ArrayMirror {
    return isObjectMirror(mirror) && mirror.subtype === "array";
}

export function isRegExpMirror(mirror: Mirror): mirror is ArrayMirror {
    return isObjectMirror(mirror) && mirror.subtype === "regexp";
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

    // 'object' test handles things like regexp/array/promise/null.
    if (isObjectMirror(mirror)) {
        return true;
    }

    throw new Error("Unhandled isTruthy case: " + JSON.stringify(mirror));
}

export function isFalsy(mirror: Mirror) {
    return !isTruthy(mirror);
}

export async function getPromiseMirrorValueAsync(mirror: PromiseMirror): Promise<Mirror> {
    const promise = getValueForMirror(mirror);
    const value = await promise;
    return await getMirrorAsync(value);
}
