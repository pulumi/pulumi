// Copyright 2016-2021, Pulumi Corporation.
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
import * as sinon from "sinon";
import { runPulumiCmd } from "../../automation";
import { asyncTest } from "../util";

describe("automation/cmd", () => {
    it("calls onOutput when provided to runPulumiCmd", asyncTest(async () => {
        const spy = sinon.spy();
        await runPulumiCmd(["version"], ".", {}, spy);

        assert.ok(spy.calledOnce);
        assert.strictEqual(spy.firstCall.firstArg, spy.lastCall.lastArg);
    }));
});

