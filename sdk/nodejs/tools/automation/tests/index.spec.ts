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

import { API, PulumiImportOptions, PulumiUpOptions } from "../output";
import { describe, it } from "mocha";
import * as assert from "assert";

describe("Command examples", () => {
    const api = new API();

    it("about", () => {
        const command = api.about({}); // An executable menu
        assert.strictEqual(command, "pulumi about");
    });

    it("config env add", () => {
        const command = api.configEnvAdd({});
        assert.strictEqual(command, "pulumi config env add");
    });

    it("template publish", () => {
        const command = api.templatePublish(
            {
                name: "test",
                version: "1.0.0",
            },
            ".", // Required flags
        );

        assert.strictEqual(
            command,
            "pulumi template publish --name test --version 1.0.0 -- .",
        );
    });

    it("import", () => {
        const options: PulumiImportOptions = {};

        const command = api.import(options, "'aws:iam/user:User'", "name", "id");
        assert.strictEqual(
            command,
            "pulumi import -- 'aws:iam/user:User' name id",
        );
    });

    it("up", () => {
        const options: PulumiUpOptions = {
            target: ["urnA", "urnB"],
        };

        const command = api.up(options, "https://pulumi.com");
        assert.strictEqual(
            command,
            "pulumi up --target urnA --target urnB -- https://pulumi.com",
        );
    });
});

