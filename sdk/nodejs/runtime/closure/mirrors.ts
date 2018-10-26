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
    type: "function" | "object" | "number" | "string";
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

type MirrorType<T> =
    T extends string ? StringMirror :
    T extends Function ? FunctionMirror : Mirror;

let currentMirrorId = 0;
const valToMirror = new Map<any, Mirror>();
const functionIdToFunc = new Map<RemoteObjectId, Function>();

export async function getMirrorAsync<T>(val: T): Promise<MirrorType<T>> {
    let mirror = valToMirror.get(val);
    if (mirror) {
        return <any>mirror;
    }

    mirror = await createMirrorAsync();
    valToMirror.set(val, mirror);
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
