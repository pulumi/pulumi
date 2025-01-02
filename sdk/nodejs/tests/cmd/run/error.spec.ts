// Copyright 2024-2024, Pulumi Corporation.
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
import * as util from "util";
import { defaultErrorMessage } from "../../../cmd/run/error";

describe("defaultErrorMessage", () => {
    function doThrow(thingToThrow: any): string {
        try {
            throw thingToThrow;
        } catch (err) {
            return defaultErrorMessage(err);
        }
    }

    class CustomError extends Error {
        constructor(message: string) {
            super(`a custom error: ${message}`);
        }
    }

    // This object has no toString, and forces an error when inspected.
    // It is very naughty.
    const veryNaughty = Object.assign(Object.create(null), {
        [util.inspect.custom]() {
            throw new Error("I'm a naughty object");
        },
    });

    const tests = [
        {
            name: "a plain string",
            input: "a plain string",
            expected: /a plain string/,
        },
        {
            name: "error with a message",
            input: new Error("error with a message"),
            expected: /Error: error with a message\n.*tests\/cmd\/run\/error.spec.*$/ms,
        },
        {
            name: "error without a message",
            input: new Error(),
            expected: /Error: \n.*tests\/cmd\/run\/error.spec.*$/ms,
        },
        {
            name: "custom error",
            input: new CustomError("hey"),
            expected: /CustomError: a custom error: hey\n.*tests\/cmd\/run\/error.spec.*$/ms,
        },
        {
            name: "object with a message",
            input: { message: "object with a message" },
            expected: /object with a message/,
        },
        {
            name: "an empty object",
            input: {},
            expected: /\[object Object\]/,
        },
        {
            name: "an empty object with no prototype",
            input: Object.create(null),
            expected: /\[Object: null prototype\] {}/,
        },
        {
            name: "object with no prototype",
            input: Object.assign(Object.create(null), {
                message: "the message is here",
            }),
            expected: /the message is here/,
        },
        {
            name: "null",
            input: null,
            expected: /^null$/,
        },
        {
            name: "undefined",
            input: undefined,
            expected: /^undefined$/,
        },
        {
            name: "very naughty",
            input: veryNaughty,
            expected: /an error occurred while inspecting an error: I'm a naughty object/,
        },
    ];

    for (const test of tests) {
        it(test.name, () => {
            assert.match(doThrow(test.input), test.expected);
        });
    }
});
