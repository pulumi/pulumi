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

import { API, PulumiCancelOptions, PulumiNewOptions } from "../output";
import { describe, it } from "mocha";
import * as assert from "assert";

describe("Command examples", () => {
    const api = new API();

    it("cancel", () => {
        const options: PulumiCancelOptions = {};
        const command = api.cancel(options, "my-stack");
        assert.strictEqual(command, "pulumi cancel --yes -- my-stack");
    });

    it("new with template", () => {
        const options: PulumiNewOptions = {};
        const command = api.new(options, "typescript");
        assert.strictEqual(command, "pulumi new --yes -- typescript");
    });

    it("new with flags", () => {
        const options: PulumiNewOptions = {
            name: "my-project",
            description: "A test project",
            stack: "dev",
            generateOnly: true,
        };
        const command = api.new(options, "typescript");
        assert.strictEqual(
            command,
            "pulumi new --yes --description A test project --generate-only --name my-project --stack dev -- typescript",
        );
    });

    it("new with config flags", () => {
        const options: PulumiNewOptions = {
            config: ["aws:region=us-east-1", "project:env=dev"],
            configPath: true,
        };
        const command = api.new(options, "aws-typescript");
        assert.strictEqual(
            command,
            "pulumi new --yes --config aws:region=us-east-1 --config project:env=dev --config-path -- aws-typescript",
        );
    });
});
