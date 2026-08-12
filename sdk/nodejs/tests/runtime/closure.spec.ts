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
import { runtime } from "../../index";

describe("closure", () => {
    describe("serializeFunction", () => {
        it("throws when running under bun", async () => {
            // Simulate bun runtime by setting process.versions.bun.
            (process.versions as any).bun = "1.0.0";
            try {
                await assert.rejects(
                    async () => {
                        await runtime.serializeFunction(() => "hello");
                    },
                    (err: Error) => {
                        assert.strictEqual(
                            err.message,
                            "Function serialization is not supported when using bun as a runtime.",
                        );
                        assert.ok(
                            err.stack?.includes("serializeFunction"),
                            `Expected stack trace to include serializeFunction, got:\n${err.stack}`,
                        );
                        return true;
                    },
                );
            } finally {
                (process.versions as any).bun = undefined;
            }
        });
    });
});
