// Copyright 2016-2026, Pulumi Corporation.
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

import * as assert from "assert";

const debuggableModulePath = require.resolve("../../runtime/debuggable");
const leakDetectorKey = "__pulumiLeakDetectorScheduled";

function restoreNewExitListeners(existingListeners: Function[]): void {
    for (const listener of process.listeners("exit")) {
        if (!existingListeners.includes(listener)) {
            process.removeListener("exit", listener as (...args: any[]) => void);
        }
    }
}

function loadDebuggableModule(): typeof import("../../runtime/debuggable") {
    delete require.cache[debuggableModulePath];
    return require(debuggableModulePath);
}

describe("runtime", () => {
    describe("debuggable", () => {
        let existingExitListeners: Function[];
        let initialMaxListeners: number;

        beforeEach(() => {
            existingExitListeners = process.listeners("exit");
            initialMaxListeners = process.getMaxListeners();
            delete (process as any)[leakDetectorKey];
            delete require.cache[debuggableModulePath];
        });

        afterEach(() => {
            restoreNewExitListeners(existingExitListeners);
            process.setMaxListeners(initialMaxListeners);
            delete (process as any)[leakDetectorKey];
            delete require.cache[debuggableModulePath];
        });

        it("registers the leak detector once across fresh module instances", async () => {
            const initialExitListenerCount = process.listenerCount("exit");
            const first = loadDebuggableModule();
            const second = loadDebuggableModule();

            await first.debuggablePromise(Promise.resolve("first"), "first");
            await second.debuggablePromise(Promise.resolve("second"), "second");

            assert.strictEqual(process.listenerCount("exit"), initialExitListenerCount + 1);
            assert.strictEqual((process as any)[leakDetectorKey], true);
        });

        it("skips registration when the process-level guard is already set", async () => {
            const initialExitListenerCount = process.listenerCount("exit");
            (process as any)[leakDetectorKey] = true;

            const debuggable = loadDebuggableModule();
            await debuggable.debuggablePromise(Promise.resolve("value"), "ctx");

            assert.strictEqual(process.listenerCount("exit"), initialExitListenerCount);
        });
    });
});
