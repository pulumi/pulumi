// Copyright 2026, Pulumi Corporation.
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
import { CustomResource, Output, runtime } from "../..";

// eslint-disable-next-line @typescript-eslint/no-var-requires
const logModule = require("../../log");

const secretValue = "s3cr3t-value-that-must-not-be-logged";

class TestResource extends CustomResource {
    public readonly password!: Output<string>;

    constructor(name: string) {
        super("test:index:resource", name, { password: undefined });
    }
}

describe("debug logging", () => {
    let messages: string[];
    let originalDebug: any;

    beforeEach(async () => {
        runtime._reset();
        messages = [];
        originalDebug = logModule.debug;
        logModule.debug = (msg: string) => {
            messages.push(msg);
            return Promise.resolve();
        };

        await runtime.setMocks({
            call: () => ({ token: secretValue }),
            newResource: (args) => ({
                id: `${args.name}_id`,
                state: { password: secretValue },
            }),
        });
    });

    afterEach(() => {
        logModule.debug = originalDebug;
    });

    it("does not log RegisterResource responses", async () => {
        const res = new TestResource("res");
        assert.strictEqual(await res.password.promise(), secretValue);

        assert.ok(messages.some((m) => m.startsWith("RegisterResource RPC finished")));
        for (const m of messages) {
            assert.ok(!m.includes(secretValue), `debug message leaked a resource output: ${m}`);
        }
    });

    it("does not log Invoke responses", async () => {
        const result = await runtime.invoke("test:index:getToken", {});
        assert.strictEqual(result.token, secretValue);

        assert.ok(messages.some((m) => m.startsWith("Invoke RPC finished")));
        for (const m of messages) {
            assert.ok(!m.includes(secretValue), `debug message leaked an invoke result: ${m}`);
        }
    });
});
