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

import {
    ArrayMirror,
    BooleanMirror,
    FunctionDetails,
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

type ValueType<TMirror> =
    TMirror extends UndefinedMirror ? undefined :
    TMirror extends NullMirror ? null :
    TMirror extends StringMirror ? string :
    TMirror extends NumberMirror ? number :
    TMirror extends BooleanMirror ? boolean :
    TMirror extends RegExpMatchArray ? RegExp :
    TMirror extends SymbolMirror ? symbol :
    TMirror extends ArrayMirror ? any[] :
    TMirror extends PromiseMirror ? Promise<any> :
    TMirror extends FunctionMirror ? Function : any;

let currentMirrorId = 0;
// const functionIdToFunc = new Map<RemoteObjectId, Function>();

const valToMirror = new Map<any, Mirror>();
const mirrorToVal = new Map<Mirror, any>();

// Not for general use.  Only when transitioning over to the inspector API.
export function getValueForMirror<TMirror extends Mirror>(mirror: TMirror): ValueType<TMirror> {
    if (!mirrorToVal.has(mirror)) {
        throw new Error("Didn't have value for mirror: " + JSON.stringify(mirror));
    }

    return mirrorToVal.get(mirror);
}

// Negative-zero has to be handled specially.  It cannot be placed in valToMirror map as it will
// collide with 0.
const negativeZeroMirror: NumberMirror = {
    __isMirror: true,
    type: "number",
    value: undefined,
    unserializableValue: "-0",
    objectId: "id" + currentMirrorId++,
};
mirrorToVal.set(negativeZeroMirror, -0);

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
    mirrorToVal.set(mirror, val);
    return <any>mirror;

    async function createMirrorAsync(): Promise<Mirror> {
        const objectId = "id" + currentMirrorId++;
        // tslint:disable-next-line:variable-name
        const __isMirror = true;

        if (typeof val === "undefined") {
            const undefinedMirror: UndefinedMirror = {
                __isMirror,
                objectId,
                type: "undefined",
            };

            return undefinedMirror;
        }

        if (typeof val === "boolean") {
            const booleanMirror: BooleanMirror = {
                __isMirror,
                objectId,
                type: "boolean",
                value: val,
            };

            return booleanMirror;
        }

        if (typeof val === "string") {
            const stringMirror: StringMirror = {
                __isMirror,
                objectId,
                type: "string",
                value: val,
            };

            return stringMirror;
        }

        if (typeof val === "number") {
            const unserializableValue =
                Object.is(val, -0) ? "-0" :
                Object.is(val, NaN) ? "NaN" :
                Object.is(val, Infinity) ? "Infinity" :
                Object.is(val, -Infinity) ? "-Infinity" : undefined;

            const numberMirror: NumberMirror = {
                __isMirror,
                objectId,
                type: "number",
                value: unserializableValue ? undefined : val,
                unserializableValue: unserializableValue,
            };

            return numberMirror;
        }

        if (typeof val === "function") {
            const funcMirror: FunctionMirror = {
                __isMirror,
                objectId,
                type: "function",
            };

            // functionIdToFunc.set(objectId, val);
            return funcMirror;
        }

        if (typeof val === "symbol") {
            const symbolMirror: SymbolMirror =  {
                __isMirror,
                objectId,
                type: "symbol",
            };

            return symbolMirror;
        }

        if (typeof val === "object") {
            // "null" | "regexp" | "promise" | "array"
            if (val === null) {
                const nullMirror: NullMirror = {
                    __isMirror,
                    objectId,
                    type: "object",
                    subtype: "null",
                    value: null,
                };

                return nullMirror;
            }

            if (val instanceof RegExp) {
                const regExpMirror: RegExpMirror = {
                    __isMirror,
                    objectId,
                    type: "object",
                    subtype: "regexp",
                    // className: "RegExp",
                };

                return regExpMirror;
            }

            if (val instanceof Promise) {
                const promiseMirror: PromiseMirror = {
                    __isMirror,
                    objectId,
                    type: "object",
                    subtype: "promise",
                    // className: "Promise",
                };

                return promiseMirror;
            }

            if (Array.isArray(val)) {
                const arrayMirror: ArrayMirror = {
                    __isMirror,
                    objectId,
                    type: "object",
                    subtype: "array",
                    // className: "Array",
                };

                return arrayMirror;
            }

            // let className = "unknown";
            // const anyVal = <any>val;
            // if (anyVal.constructor && anyVal.constructor.name) {
            //     className = anyVal.constructor.name;
            // }

            const objectMirror: ObjectMirror = {
                __isMirror,
                objectId,
                type: "object",
                // className: className,
            };

            return objectMirror;
        }

        console.log("NYI: unhandled createMirrorAsync case: " + typeof val);
        console.log("NYI: unhandled createMirrorAsync case: " + JSON.stringify(val));
        throw new Error("NYI: unhandled createMirrorAsync case: " + typeof val + " " + JSON.stringify(val));
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

export function callAccessorOn(mirror: Mirror, accessorName: string): Promise<Mirror> {
    if (!mirror.objectId) {
        throw new Error("Can't call function on mirror without an objectId: " + JSON.stringify(mirror));
    }

    if (isUndefinedOrNullMirror(mirror)) {
        throw new Error(`Trying to get member ${accessorName} off null/undefined: ${JSON.stringify(mirror)}`);
    }

    const realInstance = getValueForMirror(mirror);
    const res = realInstance[accessorName];

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

export async function getPromiseMirrorValueAsync(mirror: PromiseMirror): Promise<Mirror> {
    const promise = getValueForMirror(mirror);
    const value = await promise;
    return await getMirrorAsync(value);
}

export async function getOwnPropertyDescriptorsAsync(mirror: Mirror): Promise<MirrorPropertyDescriptor[]> {
    const obj = getValueForMirror(mirror);

    const result: MirrorPropertyDescriptor[] = [];
    for (const name of  Object.getOwnPropertyNames(obj)) {
        if (name === "__proto__") {
            // don't return prototypes here.  If someone wants one, they should call
            // Object.getPrototypeOf. Note: this is the standard behavior of
            // Object.getOwnPropertyNames.  However, the Inspector API returns these, and we want to
            // filter them out.
            continue;
        }

        const descriptor = Object.getOwnPropertyDescriptor(obj, name);
        if (!descriptor) {
            throw new Error(`Could not get descriptor for ${name} on: ${JSON.stringify(obj)}`);
        }

        result.push(await createMirrorPropertyDescriptorAsync(name, descriptor));
    }

    for (const symbol of Object.getOwnPropertySymbols(obj)) {
        const descriptor = Object.getOwnPropertyDescriptor(obj, symbol);
        if (!descriptor) {
            throw new Error(`Could not get descriptor for symbol ${symbol.toString()} on: ${JSON.stringify(obj)}`);
        }

        result.push(await createMirrorPropertyDescriptorAsync(symbol, descriptor));
    }

    return result;
}

async function createMirrorPropertyDescriptorAsync(
    nameOrSymbol: string | symbol, descriptor: PropertyDescriptor): Promise<MirrorPropertyDescriptor> {

    if (nameOrSymbol === undefined) {
        throw new Error("Was not given a name or symbol");
    }

    const copy: MirrorPropertyDescriptor = {
        configurable: descriptor.configurable,
        writable: descriptor.writable,
        enumerable: descriptor.enumerable,
    };

    if (descriptor.hasOwnProperty("value")) {
        copy.value = await getMirrorAsync(descriptor.value);
    }

    if (descriptor.get) {
        copy.get = await getMirrorAsync(descriptor.get);
    }

    if (descriptor.set) {
        copy.set = await getMirrorAsync(descriptor.set);
    }

    if (typeof nameOrSymbol === "string") {
        copy.name = await getMirrorAsync(nameOrSymbol);
    }
    else {
        copy.symbol = await getMirrorAsync(nameOrSymbol);
    }

    return copy;
}

export async function getOwnPropertyAsync(mirror: Mirror, descriptor: MirrorPropertyDescriptor): Promise<Mirror> {
    const obj = getValueForMirror(mirror);
    const nameOrSymbolMirror = getNameOrSymbol(descriptor);
    const nameOrSymbol = getValueForMirror(nameOrSymbolMirror);

    const prop = obj[nameOrSymbol];
    return await getMirrorAsync(prop);
}

export async function lookupCapturedVariableAsync(
        funcMirror: FunctionMirror, freeVariable: string, throwOnFailure: boolean): Promise<Mirror> {

    const func = getValueForMirror(funcMirror);
    const variable = await v8.lookupCapturedVariable(func, freeVariable, throwOnFailure);
    return getMirrorAsync(variable);
}

export async function getFunctionDetailsAsync(funcMirror: FunctionMirror): Promise<FunctionDetails> {
    const func = getValueForMirror(funcMirror);
    return {
        name: func.name,
        code: func.toString(),
        location: await v8.getFunctionLocation(func),
    };
}
