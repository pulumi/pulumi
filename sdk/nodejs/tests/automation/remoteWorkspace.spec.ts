// Copyright 2016-2022, Pulumi Corporation.
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

import assert from "assert";

import { isFullyQualifiedStackName } from "../../automation";

describe("isFullyQualifiedStackName", () => {
    const tests = [
        {
            name: "fully qualified",
            input: "owner/project/stack",
            expected: true,
        },
        {
            name: "undefined",
            input: undefined,
            expected: false,
        },
        {
            name: "null",
            input: null,
            expected: false,
        },
        {
            name: "empty",
            input: "",
            expected: false,
        },
        {
            name: "name",
            input: "name",
            expected: false,
        },
        {
            name: "name & owner",
            input: "owner/name",
            expected: false,
        },
        {
            name: "sep",
            input: "/",
            expected: false,
        },
        {
            name: "two seps",
            input: "//",
            expected: false,
        },
        {
            name: "three seps",
            input: "///",
            expected: false,
        },
        {
            name: "invalid",
            input: "owner/project/stack/wat",
            expected: false,
        },
    ];

    tests.forEach(test => {
        it(`${test.name}`, () => {
            const actual = isFullyQualifiedStackName(test.input!);
            assert.strictEqual(actual, test.expected);
        });
    });
});
