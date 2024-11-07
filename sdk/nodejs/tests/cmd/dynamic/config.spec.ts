// Copyright 2024-2024, Pulumi Corporation.
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
import { Config } from "../../../cmd/dynamic-provider/config";

describe("dynamic config", () => {
    it("exposes get to retrieve values", () => {
        const config = new Config({ "my-project:a": "A", "namespace:b": "B" }, "my-project");
        assert.strictEqual(config.get("my-project:a"), "A");
        assert.strictEqual(config.get("a"), "A");
        assert.strictEqual(config.get("namespace:b"), "B");
        assert.strictEqual(config.get("b"), undefined);
    });

    it("exposes require to retrieve values", () => {
        const config = new Config({ "my-project:a": "A", "namespace:b": "B" }, "my-project");
        assert.strictEqual(config.require("my-project:a"), "A");
        assert.strictEqual(config.require("a"), "A");
        assert.strictEqual(config.require("namespace:b"), "B");
        assert.throws(() => config.require("b"), /Missing required configuration key: b/);
    });
});
