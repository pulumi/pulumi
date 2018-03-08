// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";

export type MochaFunc = (err: Error) => void;

// A helper function for wrapping some of the boilerplate goo necessary to interface between Mocha's asynchronous
// testing and our TypeScript async tests.
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
export async function assertAsyncThrows(test: () => Promise<void>, message?: string): Promise<void> {
    let thrown: boolean = false;
    try {
        await test();
    }
    catch (err) {
        thrown = true;

        if (message !== undefined) {
            assert.equal(err.message, message);
        }
    }
    assert(thrown, "Function was expected to throw, but didn't");
}

