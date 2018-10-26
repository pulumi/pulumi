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
    type: "function" | "object" | "number" | "string" | "undefined";
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

    // Temporary Deviation from v8 to make transition easyer.
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
    subtype: string;
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

type MirrorType<T> =
    T extends undefined ? UndefinedMirror :
    T extends null ? NullMirror :
    T extends string ? StringMirror :
    T extends Function ? FunctionMirror : Mirror;

let currentMirrorId = 0;
const functionIdToFunc = new Map<RemoteObjectId, Function>();

const valToMirror = new Map<any, Mirror>();
const mirrorToVal = new Map<Mirror, any>();

function getValueForMirror(mirror: Mirror): any {
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

export function getFunction(funcMirror: FunctionMirror): Function {
    const func = functionIdToFunc.get(funcMirror.objectId);
    if (!func) {
        throw new Error("Couldn't find func for " + funcMirror.objectId);
    }

    return func;
}

export async function getPrototypeOfMirror(mirror: Mirror): Promise<Mirror> {
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

export function isNullMirror(mirror: Mirror): mirror is NullMirror {
    return isObjectMirror(mirror) && mirror.subtype === "null";
}

export function isObjectMirror(mirror: Mirror): mirror is ObjectMirror {
    return mirror.type === "object";
}