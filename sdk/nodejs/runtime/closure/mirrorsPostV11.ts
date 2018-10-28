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
    FunctionDetails,
    FunctionMirror,
    getNameOrSymbol,
    isMirror,
    isStringMirror,
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

const inspectorSession = new inspector.Session();
inspectorSession.connect();

const valToMirror = new Map<any, Mirror>();

const negativeZeroMirror: NumberMirror = {
    __isMirror: true,
    type: "number",
    value: undefined,
    unserializableValue: "-0",
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

    mirror = await createMirrorAsync(val);
    valToMirror.set(val, mirror);
    // mirrorToVal.set(mirror, val);
    return <any>mirror;
}

let currentValueId = 0;
async function createMirrorAsync<T>(val: T): Promise<MirrorType<T>> {
    const currentValueName = "__valueToEvaluate" + currentValueId++;
    (<any>global)[currentValueName] = val;
    try {
        return <any>await runtimeEvaluateAsync(`global.${currentValueName}`);
    }
    finally {
        delete (<any>global)[currentValueName];
    }
}

async function runtimeEvaluateAsync(expression: string): Promise<Mirror> {
    const retType = await new Promise<inspector.Runtime.EvaluateReturnType>((resolve, reject) => {
        inspectorSession.post(
            "Runtime.evaluate",
            { expression },
            (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.evaluate(${expression})": ` + retType.exceptionDetails.text);
    }

    // console.log(JSON.stringify(retType.result));
    const remoteObj = retType.result;
    return convertRemoteObject(remoteObj);
}

function convertRemoteObject(remoteObj: inspector.Runtime.RemoteObject): Mirror {
    if (!remoteObj) {
        throw new Error("Did not get passed an object to convertRemoteObject");
    }

    switch (remoteObj.type) {
        case "function":
            return convertFunction(remoteObj);
        case "object":
            return convertObject(remoteObj);
        case "number":
            return convertNumber(remoteObj);
        case "symbol":
            return convertSymbol(remoteObj);
        default:
            throw new Error("NYI: unhandled convertRemoteObject case: " + JSON.stringify(remoteObj));
    }
}

function convertObject(remoteObj: inspector.Runtime.RemoteObject): ObjectMirror {
    if (remoteObj.objectId === undefined) {
        throw new Error("Remote object did not have an objectId: " + JSON.stringify(remoteObj));
    }

    if (remoteObj.subtype === undefined) {
        return {
            __isMirror: true,
            type: "object",
            objectId: remoteObj.objectId,
        };
    }

    switch (remoteObj.subtype) {
        case "array":
            return convertArray(remoteObj, remoteObj.objectId);
        default:
            throw new Error("Unknown object subtype: " + JSON.stringify(remoteObj));
    }
}

function convertFunction(remoteObj: inspector.Runtime.RemoteObject): FunctionMirror {
    if (remoteObj.objectId === undefined) {
        throw new Error("Remote function did not have an objectId: " + JSON.stringify(remoteObj));
    }

    return {
        __isMirror: true,
        type: "function",
        objectId: remoteObj.objectId,
    };
}

function convertSymbol(remoteObj: inspector.Runtime.RemoteObject): SymbolMirror {
    if (remoteObj.objectId === undefined) {
        throw new Error("Remote function did not have an objectId: " + JSON.stringify(remoteObj));
    }

    return {
        __isMirror: true,
        type: "symbol",
        objectId: remoteObj.objectId,
    };
}

function convertArray(remoteObj: inspector.Runtime.RemoteObject, objectId: string): ArrayMirror {
    return {
        __isMirror: true,
        type: "object",
        subtype: "array",
        objectId: objectId,
    };
}

function convertNumber(remoteObj: inspector.Runtime.RemoteObject): NumberMirror {
    return {
        __isMirror: true,
        type: "number",
        value: remoteObj.value,
        unserializableValue: remoteObj.value,
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

export async function getFunctionDetailsAsync(funcMirror: FunctionMirror): Promise<FunctionDetails> {
    const { properties, internalProperties } = await runtimeGetPropertiesAsync(
        funcMirror, /*ownProperties:*/ false);

    const nameMirror = await callAccessorOn(funcMirror, "name");
    const codeMirror = await callFunctionOn(funcMirror, "toString");

    if (!isStringMirror(codeMirror)) {
        throw new Error("Did not get back a string for .toString on a function:" +
        "\n\tfunc: " + JSON.stringify(funcMirror) +
        "\n\tres:  " + JSON.stringify(codeMirror));
    }

    const name = isStringMirror(nameMirror) ? nameMirror.value || "" : "";
    const code = codeMirror.value;
    const location = getFunctionLocation(internalProperties);

    return { name, location, code };

//     for (const prop of properties) {
//         console.log(JSON.stringify(prop));
//     }
//     for (const prop of internalProperties) {
//         console.log(JSON.stringify(prop));
//     }
}

function getFunctionLocation(properties: inspector.Runtime.InternalPropertyDescriptor[]) {
    const functionLocation = properties.find(p => p.name === "[[FunctionLocation]]");
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

async function runtimeGetPropertiesAsync(
        mirror: Mirror,
        ownProperties: boolean | undefined) {

    if (!isMirror(mirror)) {
        throw new Error("Asking for the properties of non-mirror: " + JSON.stringify(mirror));
    }

    const objectId = mirror.objectId;
    if (objectId === undefined) {
        throw new Error("Asking for the properties of mirror without objectId: " + JSON.stringify(mirror));
    }

    const retType = await new Promise<inspector.Runtime.GetPropertiesReturnType>((resolve, reject) => {
        inspectorSession.post(
            "Runtime.getProperties",
            { objectId, ownProperties },
            (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.getProperties(${objectId}, ${ownProperties})": `
            + retType.exceptionDetails.text);
    }

    return { internalProperties: retType.internalProperties || [], properties: retType.result };
}
