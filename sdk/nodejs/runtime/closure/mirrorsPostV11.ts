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

// tslint:disable:max-line-length
import * as inspector from "inspector";

import {
    ArrayMirror,
    BooleanMirror,
    FunctionDetails,
    FunctionMirror,
    getNameOrSymbol,
    isBooleanMirror,
    isFunctionMirror,
    isMirror,
    isStringMirror,
    isSymbolMirror,
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

const objectGroup = "pulumiObjectGroup";

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
            { expression, objectGroup },
            (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.evaluate(${expression})": ` + retType.exceptionDetails.text);
    }

    // console.log(JSON.stringify(retType.result));
    const remoteObj = retType.result;
    return await convertRemoteObjectAsync(remoteObj);
}

const convertedObjects: Mirror[] = [];

async function convertRemoteObjectAsync(remoteObj: inspector.Runtime.RemoteObject): Promise<Mirror> {
    if (!remoteObj) {
        throw new Error("Did not get passed an object to convertRemoteObject");
    }

    const converted = convert();

    // Inspector can return different mirrors for the same object.  See if this maps to an existing
    // object we've already converted, and return that instance instead so we preserve identity.
    if (converted.objectId !== undefined) {
        const existingMirror = await tryFindExistingMirrorAsync();
        if (existingMirror) {
            return existingMirror;
        }

        convertedObjects.push(converted);
    }

    return converted;

    function convert() {
        switch (remoteObj.type) {
            case "function":
                return convertFunction();
            case "object":
                return convertObject();
            case "boolean":
                return convertBoolean();
            case "number":
                return convertNumber();
            case "string":
                return convertString();
            case "symbol":
                return convertSymbol();
            case "undefined":
                return convertUndefined();
            default:
                throw new Error("NYI: unhandled convertRemoteObject case: " + JSON.stringify(remoteObj));
        }
    }

    async function tryFindExistingMirrorAsync(): Promise<Mirror | undefined> {
        for (const existing of convertedObjects) {
            const retType = await new Promise<inspector.Runtime.CallFunctionOnReturnType>((resolve, reject) => {
                inspectorSession.post("Runtime.callFunctionOn", {
                    objectId: converted.objectId,
                    functionDeclaration: `function(a, b) {
                        return a === b;
                    }`,
                    arguments: [{ objectId: converted.objectId }, { objectId: existing.objectId }],
                },
                (err, ret) => err ? reject(err) : resolve(ret));
            });

            if (retType.exceptionDetails) {
                throw new Error(
`Error calling Runtime.callFunctionOn(${JSON.stringify(converted)} === ${JSON.stringify(existing)})` +
retType.exceptionDetails.text);
            }

            if (retType.result.type !== "boolean") {
                throw new Error(
`Runtime.callFunctionOn(${JSON.stringify(converted)} === ${JSON.stringify(existing)}) returned: ` +
JSON.stringify(retType.result));
            }

            if (retType.result.value) {
                return existing;
            }
        }

        return undefined;
    }

    function convertObject(): ObjectMirror {
        if (remoteObj.subtype === "null") {
            return convertNull();
        }

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
                return convertArray(remoteObj.objectId);
            default:
                throw new Error("Unknown object subtype: " + JSON.stringify(remoteObj));
        }
    }

    function convertFunction(): FunctionMirror {
        if (remoteObj.objectId === undefined) {
            throw new Error("Remote function did not have an objectId: " + JSON.stringify(remoteObj));
        }

        return {
            __isMirror: true,
            type: "function",
            objectId: remoteObj.objectId,
        };
    }

    function convertSymbol(): SymbolMirror {
        if (remoteObj.objectId === undefined) {
            throw new Error("Remote function did not have an objectId: " + JSON.stringify(remoteObj));
        }

        return {
            __isMirror: true,
            type: "symbol",
            objectId: remoteObj.objectId,
        };
    }

    function convertArray(objectId: string): ArrayMirror {
        return {
            __isMirror: true,
            type: "object",
            subtype: "array",
            objectId: objectId,
        };
    }

    function convertBoolean(): BooleanMirror {
        return {
            __isMirror: true,
            type: "boolean",
            value: remoteObj.value,
        };
    }

    function convertNumber(): NumberMirror {
        return {
            __isMirror: true,
            type: "number",
            value: remoteObj.value,
            unserializableValue: remoteObj.value,
        };
    }

    function convertString(): StringMirror {
        return createStringMirror(remoteObj.value);
    }

    function convertUndefined(): UndefinedMirror {
        return createUndefinedMirror();
    }

    function convertNull(): NullMirror {
        return {
            __isMirror: true,
            type: "object",
            subtype: "null",
            value: null,
        };
    }
}

function createStringMirror(value: string): StringMirror {
    return {
        __isMirror: true,
        type: "string",
        value: value,
    };
}

function createUndefinedMirror(): UndefinedMirror {
    return {
        __isMirror: true,
        type: "undefined",
    };
}

const mirrorToPrototypeMap = new Map<Mirror, Mirror>();

export async function getPrototypeOfMirrorAsync(mirror: Mirror): Promise<Mirror> {
    // Calling Object.getPrototypeOf(mirror_value)

    let result = mirrorToPrototypeMap.get(mirror);
    if (!result) {
        const objectMirror = await getMirrorAsync(Object);
        result = await callFunctionOn(objectMirror, "getPrototypeOf", [mirror]);
        mirrorToPrototypeMap.set(mirror, result);
    }

    console.log(`Prototype of ${JSON.stringify(mirror)} was ${JSON.stringify(result)}`);
    return result;
}

export async function callFunctionOn(mirror: Mirror, funcName: string, mirrorArgs: Mirror[] = []): Promise<Mirror> {
    const objectId = getObjectId(mirror);
    const args: inspector.Runtime.CallArgument[] = mirrorArgs.map(a => ({
        objectId: getObjectId(a),
    }));

    const retType = await new Promise<inspector.Runtime.CallFunctionOnReturnType>((resolve, reject) => {
        inspectorSession.post("Runtime.callFunctionOn", {
            objectId: objectId,
            functionDeclaration: `function (...args) {
                return this["${funcName}"](...args);
            }`,
            arguments: args,
        }, (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling Runtime.callFunctionOn(${JSON.stringify(mirror)}, "${funcName}"): `
            + retType.exceptionDetails.text);
    }

    return await convertRemoteObjectAsync(retType.result);
}

export async function callAccessorOn(mirror: Mirror, accessorName: string): Promise<Mirror> {
    if (!mirror.objectId) {
        return createUndefinedMirror();
    }

    const retType = await new Promise<inspector.Runtime.CallFunctionOnReturnType>((resolve, reject) => {
        inspectorSession.post("Runtime.callFunctionOn", {
            objectId: mirror.objectId,
            functionDeclaration: `function () {
                return this["${accessorName}"];
            }`,
            // arguments: [],
        }, (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling Runtime.callAccessorOn(${JSON.stringify(mirror)}, "${accessorName}"): `
            + retType.exceptionDetails.text);
    }

    return await convertRemoteObjectAsync(retType.result);
}

export async function getPromiseMirrorValueAsync(mirror: PromiseMirror): Promise<Mirror> {
    throw new Error("getPromiseMirrorValueAsync NYI");
}

export async function getOwnPropertyDescriptorsAsync(mirror: Mirror): Promise<MirrorPropertyDescriptor[]> {
    const objectId = getObjectId(mirror);

    const params: inspector.Runtime.GetPropertiesParameterType = {
        objectId: objectId,
        ownProperties: true,
    };

    const retType = await new Promise<inspector.Runtime.GetPropertiesReturnType>((resolve, reject) => {
        inspectorSession.post(
            "Runtime.getProperties", params, (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling Runtime.getProperties(${JSON.stringify(mirror)}`
            + retType.exceptionDetails.text);
    }

    const result: MirrorPropertyDescriptor[] = [];
    for (const descriptor of retType.result) {
        result.push(await convertDescriptorAsync(descriptor));
    }

    return result;

    async function convertDescriptorAsync(descriptor: inspector.Runtime.PropertyDescriptor) {
        if (descriptor.name === undefined && descriptor.symbol === undefined) {
            throw new Error("Got descriptor without name or symbol: " + JSON.stringify(descriptor));
        }

        const name = descriptor.symbol !== undefined ? undefined : createStringMirror(descriptor.name);
        const symbol = descriptor.symbol !== undefined ? await convertRemoteObjectAsync(descriptor.symbol) : undefined;
        const get = descriptor.get !== undefined ? await convertRemoteObjectAsync(descriptor.get) : undefined;
        const set = descriptor.set !== undefined ? await convertRemoteObjectAsync(descriptor.set) : undefined;
        const value = descriptor.value !== undefined ? await convertRemoteObjectAsync(descriptor.value) : undefined;

        if (symbol && !isSymbolMirror(symbol)) {
            throw new Error("Did not convert .symbol to a SymbolMirror: " + JSON.stringify(symbol));
        }

        if (get && !isFunctionMirror(get)) {
            throw new Error("Did not convert .get to a FunctionMirror: " + JSON.stringify(get));
        }

        if (set && !isFunctionMirror(set)) {
            throw new Error("Did not convert .set to a FunctionMirror: " + JSON.stringify(set));
        }

        const converted: MirrorPropertyDescriptor = {
            configurable: descriptor.configurable,
            enumerable: descriptor.enumerable,
            writable: descriptor.writable,
            name: name,
            symbol: symbol,
            get: get,
            set: set,
            value: value,
        };

        return converted;
    }
}

export async function getOwnPropertyAsync(mirror: Mirror, descriptor: MirrorPropertyDescriptor): Promise<Mirror> {
    const objectId = getObjectId(mirror);

    const args: inspector.Runtime.CallArgument[] = [{
        value: descriptor.name ? descriptor.name.value : undefined,
        objectId: descriptor.symbol ? descriptor.symbol.objectId : undefined,
    }];
    const params: inspector.Runtime.CallFunctionOnParameterType = {
        objectId: objectId,
        functionDeclaration: `function(a) {
            return this[a];
        }`,
        arguments: args,
    };

    const retType = await new Promise<inspector.Runtime.CallFunctionOnReturnType>((resolve, reject) => {
        inspectorSession.post("Runtime.callFunctionOn", params, (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling Runtime.callFunctionOn(${JSON.stringify(mirror)}, ${JSON.stringify(descriptor)}): `
            + retType.exceptionDetails.text);
    }

    return await convertRemoteObjectAsync(retType.result);
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

    console.log(`details for ${JSON.stringify(funcMirror)}: '${name}', ${code}`);
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

function getObjectId(mirror: Mirror): string {
    if (!isMirror(mirror)) {
        throw new Error("Asking for objectId of a non-mirror: " + JSON.stringify(mirror));
    }

    const objectId = mirror.objectId;
    if (objectId === undefined) {
        throw new Error("Asking for the objectId of mirror without objectId: " + JSON.stringify(mirror));
    }

    return objectId;
}

async function runtimeGetPropertiesAsync(
        mirror: Mirror,
        ownProperties: boolean | undefined) {

    const objectId = getObjectId(mirror);

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
