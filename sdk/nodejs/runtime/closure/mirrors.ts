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
// import * as inspector from "inspector";
// const session = new inspector.Session();
// session.connect();

// session.post("Runtime.getProperties")

type RemoteObjectId = string;

type UnserializableValue = string;

export interface Mirror {
    /** Object type. */
    type: "function" | "object" | "number" | "string" | "undefined" | "boolean" | "symbol";
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

    __isMirror: true;
}

export interface FunctionMirror extends Mirror {
    type: "function";
    className: "Function";
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

export interface SymbolMirror extends Mirror {
    type: "symbol";
    objectId: RemoteObjectId;

    // properties that never appear
    subtype?: never;
    className?: never;
    unserializableValue?: never;
    description?: never;
    value?: never;
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
    T extends symbol ? SymbolMirror :
    T extends Array<infer A> ? ArrayMirror :
    T extends Promise<infer B> ? PromiseMirror :
    T extends Function ? FunctionMirror : Mirror;

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
    // We should never be passed a Mirror here.  It indicates that somehow during serialization we
    // creates a Mirror, then pointed at that Mirror with something, then tried to actually
    // serialize the Mirror (instead of the value the Mirror represents).  This should not be
    // possible and indicates a bug somewhere in serialization.  Catch early to prevent any bugs.
    if (isMirror(val)) {
        throw new Error("Should never be trying to get the Mirror for a Mirror: " + JSON.stringify(val));
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
        const mirrorId = "id" + currentMirrorId++;

        if (typeof val === "string") {
            const stringMirror: StringMirror = {
                __isMirror: true,
                type: "string",
                value: val,
            };

            return stringMirror;
        }

        if (val instanceof Function) {
            const funcMirror: FunctionMirror = {
                __isMirror: true,
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

export function callAccessorOn(mirror: Mirror, accessorName: string): Promise<Mirror> {
    if (!mirror.objectId) {
        throw new Error("Can't call function on mirror without an objectId: " + JSON.stringify(mirror));
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

export async function getMirrorMemberAsync(mirror: Mirror, memberName: string): Promise<Mirror> {
    if (isUndefinedOrNullMirror(mirror)) {
        throw new Error(`Trying to get member ${memberName} off null/undefined: ${JSON.stringify(mirror)}`);
    }

    const val = getValueForMirror(mirror);
    const member = val[memberName];
    return getMirrorAsync(member);
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

export function isUndefinedOrNullMirror(mirror: Mirror) {
    return isUndefinedMirror(mirror) || isNullMirror(mirror);
}

export function isStringValue(mirror: Mirror | undefined, val: string): boolean {
    return isStringMirror(mirror) && mirror.value === val;
}

export async function getPromiseMirrorValueAsync(mirror: PromiseMirror): Promise<Mirror> {
    const promise = getValueForMirror(mirror);
    const value = await promise;
    return await getMirrorAsync(value);
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

async function getPropertyAsync(obj: any, name: string): Promise<any> {
    return obj[name];
}

export function getNameOrSymbol(descriptor: MirrorPropertyDescriptor): SymbolMirror | StringMirror {
    if (descriptor.symbol === undefined && descriptor.name === undefined) {
        throw new Error("Descriptor didn't have symbol or name: " + JSON.stringify(descriptor));
    }

    return descriptor.symbol || descriptor.name!!;
}
