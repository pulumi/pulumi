// Copyright 2016-2025, Pulumi Corporation.
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

import "mocha";
import * as assert from "assert";
import * as pulumi from "../index";
import { getStore } from "../runtime/state";

describe("program exports via runtime stack", () => {
    before(() => {
        pulumi.runtime.setMocks({
            call: () => ({}),
            newResource: (args) => ({ id: `${args.name}_id`, state: args.inputs }),
        });
    });

    it("resolves exports to plain values and records export map", async () => {
        const outs = await pulumi.runtime.runInPulumiStack(async () => {
            const nested = pulumi.output(Promise.resolve({ n: 1 }));
            return {
                a: 42,
                b: pulumi.output(99),
                c: { x: nested, y: "z" },
            } as const;
        });

        assert.deepStrictEqual(outs, { a: 42, b: 99, c: { x: { n: 1 }, y: "z" } });

        const exportMap = getStore().currentExportMap as Record<string, any> | undefined;
        assert.ok(exportMap, "expected currentExportMap to be set after stack outputs registration");
        assert.deepStrictEqual(exportMap, { a: 42, b: 99, c: { x: { n: 1 }, y: "z" } });
    });
});
