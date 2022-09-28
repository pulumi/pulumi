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

import { Resource } from "./resource";
import * as settings from "./runtime/settings";
import * as utils from "./utils";
import { Unwrap, Output, Input, Inputs, all, output } from "./output";
import { runPulumiCmd } from "./automation";

export function jsonStringify(obj: Inputs, replacer?: (this: any, key: string, value: any) => any, space?: string | number): Output<string> {
    return output(obj).apply(o => JSON.stringify(o, replacer, space));
}

export function jsonParse(str: Input<string>, reviver?: (this: any, key: string, value: any) => any): Output<any> {
    return output(str).apply(s => JSON.parse(s, reviver));
}

export function stringSplit(str: Input<string>, separator: { [Symbol.split](string: string, limit?: number): string[]; }, limit?: number): Output<string[]> {
    return output(str).apply(s => s.split(separator, limit));
}

export function ifThenElse<T,U>(v: Input<T>, c: (x: Unwrap<T>)=> boolean, t: (x: Unwrap<T>)=> U, e: (x: Unwrap<T>)=> U): Output<U> {
    return output(v).apply(x => c(x) ? t(x) : e(x));
}

// export function concat<T>(p0: Input<T[]>, ...pRest: Input<T[]>[]): Output<Unwrap<T>[]> {
//     return all([p0, ...pRest]).apply(([v0, ...vRest]) => v0.concat(...vRest));
// }

export function arrayConcat<T>(...pRest: Input<T[]>[]): Output<Unwrap<T>[]> {
    return all(pRest).apply(vrest => [].concat(...(vrest as any)));
}

export function toString(v: Input<any>): Output<string> {
    return output(v).apply(String);
}

export function base64decode(s: Input<string>): Output<string> {
    return output(s).apply(x => Buffer.from(x, "base64").toString("ascii"));
}

export function base64encode(s: Input<string>): Output<string> {
    return output(s).apply(x => Buffer.from(x, "ascii").toString("base64"));
}

export function nullCoalesce<T>(x: Input<T | undefined | null>, d: Unwrap<T>): Output<Unwrap<T>> {
    return <any>output(x).apply(v => v ?? d);
}