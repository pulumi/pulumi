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

import * as assert from "assert";
import { runPulumiCmd } from "../../automation";
import { asyncTest } from "../util";

describe("automation/cmd", () => {
    it("calls onOutput when provided to runPulumiCmd", asyncTest(async () => {
        let output = "";
        let numCalls = 0;
        await runPulumiCmd(["--help"], ".", {}, (data: string) => {
            output += data;
            numCalls += 1;
        });
        assert.ok(numCalls > 0, `expected numCalls > 0, got ${numCalls}`);
        assert.match(output, new RegExp("Usage[:]"));
        assert.match(output, new RegExp("[-][-]verbose"));
    }));
});
