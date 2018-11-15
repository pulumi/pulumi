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

describe("string", () => {
    it("awaits outputs correctly", asyncTest(async () => {
        const outer = "outer";
        const inner = "inner";

        const output1 = resource.output(Promise.resolve(outer));
        const output2 = output1.apply(_ => resource.output(Promise.resolve(inner)));
        const output3 = ps`${output1} ${output2}`;

        const value = await output3.promise();
        assert.equal(value, `${outer} ${inner}`);
    }));
    it("handles undefined and null correctly", asyncTest(async () => {
        const outer = undefined;
        const inner = null;

        const output1 = resource.output(Promise.resolve(outer));
        const output2 = output1.apply(_ => resource.output(Promise.resolve(inner)));
        const output3 = ps`${output1} ${output2}`;

        const value = await output3.promise();
        assert.equal(value, `${outer} ${inner}`);
    }));
    it("handles complex objects correctly", asyncTest(async () => {
        const outer = [ "foo" ];
        const inner = { "foo": "bar" };

        const output1 = resource.output(Promise.resolve(outer));
        const output2 = output1.apply(_ => resource.output(Promise.resolve(inner)));
        const output3 = ps`${output1} ${output2}`;

        const value = await output3.promise();
        assert.equal(value, `${outer} ${inner}`);
    }));
});
