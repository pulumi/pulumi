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

class TestReadResource extends CustomResource {
    public readonly value!: Output<string>;

    constructor(name: string, value: string) {
        super("test:index:read", name, { value }, { id: `${name}_id` });
    }
}

describe("runtime", () => {
    beforeEach(() => {
        runtime._reset();
        runtime._resetResourcePackages();
        runtime._resetResourceModules();
    });

    describe("readResource", () => {
        it("resolves the inputs as outputs without an engine", async () => {
            runtime._setProject("project");
            runtime._setStack("stack");

            const res = new TestReadResource("res", "hello");

            assert.strictEqual(await res.value.promise(), "hello");
        });

        it("rejects the outputs when the read fails", async () => {
            await runtime.setMocks({
                call: () => {
                    throw new Error("unexpected call");
                },
                newResource: () => {
                    throw new Error("failed read");
                },
            });

            const res = new TestReadResource("res", "hello");

            await assert.rejects(res.value.promise(), /failed read/);
        });
    });
});
