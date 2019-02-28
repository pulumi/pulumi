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

/**
 * Common code for doing RTTI typechecks.  RTTI is done by having a boolean property on an object
 * with a special name (like "__resource" or "__asset").  This function checks that the object
 * exists, has a **boolean** property with that name, and that that boolean property has the value
 * of 'true'.  Checking that property is 'boolean' helps ensure that this test works even on proxies
 * that synthesize properties dynamically (like Output).  Checking that the property has the 'true'
 * value isn't strictly necessary, but works to make sure that the impls are following a common
 * pattern.
 */
/** @internal */
export function isInstance<T>(obj: any, name: keyof T): obj is T {
    return hasTrueBooleanMember(obj, name);
}

/** @internal */
export function hasTrueBooleanMember(obj: any, memberName: string | number | symbol): boolean {
    if (obj === undefined || obj === null) {
        return false;
    }

    const val = obj[memberName];
    if (typeof val !== "boolean") {
        return false;
    }

    return val === true;
}
