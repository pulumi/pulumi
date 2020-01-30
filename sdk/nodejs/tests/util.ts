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

import * as assert from "assert";

/** @internal */
export type MochaFunc = (err: Error) => void;

// A helper function for wrapping some of the boilerplate goo necessary to interface between Mocha's asynchronous
// testing and our TypeScript async tests.
/** @internal */
export function asyncTest(test: () => Promise<void>): (func: MochaFunc) => void {
    return (done: (err: any) => void) => {
        const go = async () => {
            let caught: Error | undefined;
            try {
                await test();
            }
            catch (err) {
                caught = err;
            }
            finally {
                done(caught);
            }
        };
        go();
    };
}

// A helper function for asynchronous tests that throw.
/** @internal */
export async function assertAsyncThrows(test: () => Promise<void>): Promise<string> {
    try {
        await test();
    }
    catch (err) {
        return err.message;
    }

    assert(false, "Function was expected to throw, but didn't");
    return "";
}
