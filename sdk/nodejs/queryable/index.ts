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
import { Resource } from "../resource";

/**
 * ResolvedResource is a `Resource` with all fields containing `Output` values fully resolved. This
 * is useful primarily when we're querying over resource outputs (e.g., using
 * `pulumi.runtime.listResourceOutputs`), and we expect all values to be present and fully-resolved.
 */
export type ResolvedResource<T extends Resource> = Omit<Resolved<T>, "urn" | "getProvider">;

export type Resolved<T> = T extends Promise<infer U1>
    ? ResolvedSimple<U1>
    : T extends OutputInstance<infer U2>
    ? ResolvedSimple<U2>
    : ResolvedSimple<T>;

type primitive = string | number | boolean | undefined | null;

type ResolvedSimple<T> = T extends primitive
    ? T
    : T extends Array<infer U>
    ? ResolvedArray<U>
    : T extends Function
    ? never
    : T extends object
    ? ResolvedObject<T>
    : never;

interface ResolvedArray<T> extends Array<Resolved<T>> {}

type ResolvedObject<T> = ModifyOptionalProperties<{ [P in keyof T]: Resolved<T[P]> }>;

type RequiredKeys<T> = { [P in keyof T]: undefined extends T[P] ? never : P }[keyof T];
type OptionalKeys<T> = { [P in keyof T]: undefined extends T[P] ? P : never }[keyof T];

type ModifyOptionalProperties<T> = { [P in RequiredKeys<T>]: T[P] } &
    { [P in OptionalKeys<T>]?: T[P] };
