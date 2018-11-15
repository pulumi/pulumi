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

// tslint:disable

import * as assert from "assert";
import * as resource from "../resource";
import * as runtime from "../runtime";
import { string as ps } from "../string";
import { asyncTest } from "./util";

async function runCases(outer: any, inner: any): Promise<void> {
    const output1 = resource.output(Promise.resolve(outer));
    const output2 = output1.apply(_ => resource.output(Promise.resolve(inner)));

    const cases = [
        {
            expected: `${outer}`,
            output: ps`${output1}`,
        },
        {
            expected: `${inner}`,
            output: ps`${output2}`,
        },
        {
            expected: `${outer} ${inner}`,
            output: ps`${output1} ${output2}`,
        },
        {
            expected: `my ${outer} value and my ${inner} value`,
            output: ps`my ${output1} value and my ${output2} value`,
        },
        {
            expected: `my ${outer}`,
            output: ps`my ${output1}`,
        },
        {
            expected: `${outer} value`,
            output: ps`${output1} value`,
        },
    ];

    for (const c of cases) {
        assert.equal(await c.output.promise(), c.expected);
    }
}

describe("string", () => {
    it("awaits outputs correctly", asyncTest(() => runCases("outer", "inner")));
    it("handles undefined and null correctly", asyncTest(() => runCases(undefined, null)));
    it("handles complex objects correctly", asyncTest(() => runCases([ "foo" ], { "foo": "bar" })));
});
