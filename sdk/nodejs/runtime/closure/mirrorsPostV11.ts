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
// Negative-zero has to be handled specially.  It cannot be placed in valToMirror map as it will
// collide with 0.

import * as inspector from "inspector";

import {
    ArrayMirror,
    BooleanMirror,
    FunctionMirror,
    getNameOrSymbol,
    isMirror,
    isUndefinedOrNullMirror,
    Mirror,
    MirrorPropertyDescriptor,
    MirrorType,
    NullMirror,
    NumberMirror,
    ObjectMirror,
    PromiseMirror,
    RegExpMirror,
    StringMirror,
    SymbolMirror,
    UndefinedMirror,
} from "./mirrors";

const session = new inspector.Session();
session.connect();

async function getFunctionLocation(func: Function) {
    const functionMirror = await getFunctionMirrorAsync(func);
    const { properties, internalProperties } = await runtimeGetPropertiesAsync(
        functionMirror.objectId, /*ownProperties:*/ false);
    for (const prop of properties) {
        console.log(JSON.stringify(prop));
    }
    for (const prop of internalProperties) {
        console.log(JSON.stringify(prop));
    }
    const functionLocation = internalProperties.find(p => p.name === "[[FunctionLocation]]");
    if (functionLocation && functionLocation.value && functionLocation.value.value) {
        const value = functionLocation.value.value;
        const scriptId = value.scriptId;
        const line = value.lineNumber;
        const column = value.columnNumber;
        const file = /*scriptIdToUrlMap.get(scriptId) ||*/ "";
        return { file, line, column };
    }
    return { file: "", line: 0, column: 0 };
}

let currentMirrorId = 0;

const valToMirror = new Map<any, Mirror>();

const negativeZeroMirror: NumberMirror = {
    __isMirror: true,
    type: "number",
    value: undefined,
    unserializableValue: "-0",
    objectId: "id" + currentMirrorId++,
};
// mirrorToVal.set(negativeZeroMirror, -0);

export async function getMirrorAsync<T>(val: T): Promise<MirrorType<T>> {
    // We should never be passed a Mirror here.  It indicates that somehow during serialization we
    // creates a Mirror, then pointed at that Mirror with something, then tried to actually
    // serialize the Mirror (instead of the value the Mirror represents).  This should not be
    // possible and indicates a bug somewhere in serialization.  Catch early to prevent any bugs.
    if (isMirror(val)) {
        throw new Error("Should never be trying to get the Mirror for a Mirror: " + JSON.stringify(val));
    }

    if (Object.is(val, -0)) {
        return <any>negativeZeroMirror;
    }

    let mirror = valToMirror.get(val);
    if (mirror) {
        return <any>mirror;
    }

    mirror = await createMirrorAsync();
    valToMirror.set(val, mirror);
    // mirrorToVal.set(mirror, val);
    return <any>mirror;

    async function createMirrorAsync(): Promise<Mirror> {


        if (typeof val === "function") {


            const funcMirror: FunctionMirror = {
                __isMirror,
                objectId,
                type: "function",
                className: "Function",
                description: val.toString(),
                name: val.name,
                location: await v8.getFunctionLocationAsync(val),
            };

            // functionIdToFunc.set(objectId, val);
            return funcMirror;
        }


        console.log("NYI: unhandled createMirrorAsync case: " + typeof val);
        console.log("NYI: unhandled createMirrorAsync case: " + JSON.stringify(val));
        throw new Error("NYI: unhandled createMirrorAsync case: " + typeof val + " " + JSON.stringify(val));
    }
}

export async function getPrototypeOfMirrorAsync(mirror: Mirror): Promise<Mirror> {
    throw new Error("getPrototypeOfMirrorAsync NYI");
}

export function callFunctionOn(mirror: Mirror, funcName: string, args: Mirror[] = []): Promise<Mirror> {
    throw new Error("callFunctionOn NYI");
}

export function callAccessorOn(mirror: Mirror, accessorName: string): Promise<Mirror> {
    throw new Error("callAccessorOn NYI");
}

export async function getPromiseMirrorValueAsync(mirror: PromiseMirror): Promise<Mirror> {
    throw new Error("getPromiseMirrorValueAsync NYI");
}

export async function getOwnPropertyDescriptorsAsync(mirror: Mirror): Promise<MirrorPropertyDescriptor[]> {
    throw new Error("getOwnPropertyDescriptorsAsync NYI");
}

export async function getOwnPropertyAsync(mirror: Mirror, descriptor: MirrorPropertyDescriptor): Promise<Mirror> {
    throw new Error("getOwnPropertyAsync NYI");
}

export async function lookupCapturedVariableAsync(
        funcMirror: FunctionMirror, freeVariable: string, throwOnFailure: boolean): Promise<Mirror> {

    throw new Error("lookupCapturedVariableAsync NYI");
}
