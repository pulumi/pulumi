// Copyright 2025-2025, Pulumi Corporation.
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

// This file will be loaded by Node.js to setup TypeScript transpilation import
// hooks when we're running in automatic ESM mode.

import * as tsn from "ts-node";

let options: any = null;

/**
 * Called by Node.js when the hooks are registered in `run.ts`.
 */
export async function initialize(args: any) {
    options = args;
}

const makeHooks = () => {
    const service = tsn.register(options);
    // @ts-ignore we're using a version of ts-node that has ts-node/esm available.
    return tsn.createEsmHooks(service);
};

export const { resolve, load, getFormat, transformSource } = makeHooks();
