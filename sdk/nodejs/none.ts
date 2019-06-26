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

import * as utils from "./utils";

export class None {
    /** @internal */
    // tslint:disable-next-line:variable-name
    public __isPulumiNone = true;

    public static isNone(obj: any): obj is None {
        return utils.isInstance<None>(obj, "__isPulumiNone");
    }
}

/**
 * Sentinel value allowed by some APIs to indicate that no value should be created.
 * For example, some APIs want to allow any of these forms:
 *
 * ```ts
 * const r1 = new X({ });                   // 'role' not provided, create a suitable default.
 * const r1 = new X({ role: undefined });   // same as above, just explicit.
 * const r3 = new X({ role: new Role() });  // explicit 'role' provided.
 * const r4 = new X({ role: pulumi.none }); // do not use or create a role here.
 * ```
 *
 * APIs can then be typed like so:
 *
 * ```ts
 * interface XArgs
 * {
 *     role: Role | None | undefined;
 * }
 * ```
 *
 * To indicate the allowable values.
 */
export const none = new None();
