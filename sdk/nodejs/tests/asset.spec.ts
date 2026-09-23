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

/* eslint-disable */

import * as assert from "assert";
import { output, Output } from "../output";
import { StringAsset, stringAssetOutput } from "../asset";

describe("stringAssetOutput", () => {
    it("creates a StringAsset from a plain string", async () => {
        const result = stringAssetOutput("hello");
        assert.ok(result instanceof Output);
        const val = await result.promise();
        assert.ok(val instanceof StringAsset);
        assert.strictEqual(await val.text, "hello");
    });

    it("creates a StringAsset from a Promise<string>", async () => {
        const result = stringAssetOutput(Promise.resolve("world"));
        const val = await result.promise();
        assert.ok(val instanceof StringAsset);
        assert.strictEqual(await val.text, "world");
    });

    it("creates a StringAsset from an Output<string>", async () => {
        const strOut = output("from-output");
        const result = stringAssetOutput(strOut);
        assert.ok(result instanceof Output);
        const val = await result.promise();
        assert.ok(val instanceof StringAsset);
        assert.strictEqual(await val.text, "from-output");
    });

    it("preserves known/unknown state from the input output", async () => {
        const knownOut = output("known");
        const result = stringAssetOutput(knownOut);
        const isKnown = await result.isKnown;
        assert.strictEqual(isKnown, true);
    });
});
