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

import { OutputInstance } from "../index";
import { CustomResource } from "../resource";

type KeyOf = string | number | symbol;
type Diff<T extends KeyOf, U extends KeyOf> = ({ [P in T]: P } &
    { [P in U]: never } & { [x: string]: never })[T];
type Omit<T, K extends KeyOf> = Pick<T, Diff<keyof T, K>>;

export type QueryableCustomResource<T extends CustomResource> = Omit<
    Queryable<T>,
    "urn" | "getProvider"
>;

export type Queryable<T> = T extends Promise<infer U1>
    ? QueryableSimple<U1>
    : T extends OutputInstance<infer U2>
    ? QueryableSimple<U2>
    : QueryableSimple<T>;

type primitive = string | number | boolean | undefined | null;

type QueryableSimple<T> = T extends primitive
    ? T
    : T extends Array<infer U>
    ? QueryableArray<T>
    : T extends Function
    ? never
    : T extends object
    ? QueryableObject<T>
    : never;

interface QueryableArray<T> extends Array<Queryable<T>> {}

type QueryableObject<T> = ModifyOptionalProperties<{ [P in keyof T]: Queryable<T[P]> }>;

type RequiredKeys<T> = { [P in keyof T]: undefined extends T[P] ? never : P }[keyof T];
type OptionalKeys<T> = { [P in keyof T]: undefined extends T[P] ? P : never }[keyof T];

type ModifyOptionalProperties<T> = { [P in RequiredKeys<T>]: T[P] } &
    { [P in OptionalKeys<T>]?: T[P] };
