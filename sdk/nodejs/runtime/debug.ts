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

import * as inspector from "inspector";

export interface DebugOptions {
    break?: boolean;
}

const waitForDebugger: () => void = (<any>inspector).waitForDebugger;

/**
 * Start a debugger in the current Pulumi Node.js program.  Displays a console message indicating how to attach to the
 * program.
 *
 * @param opts Optionally specify whether to break immediately, or continue on to a breakpoint.
 */
export function debug(opts?: DebugOptions) {
    if (!waitForDebugger) {
        throw new Error("Debugging for Pulumi Node.js programs is supported on Node.js v12.7.0 and later.");
    }
    inspector.open();
    console.warn(`Waiting for debugger to attach to Node.js process: ${process.pid}`);
    waitForDebugger();
    if (opts?.break) {
        // tslint:disable-next-line:no-debugger
        debugger;
    }
}
