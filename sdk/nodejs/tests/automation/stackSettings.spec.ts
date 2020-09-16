// Copyright 2016-2020, Pulumi Corporation.
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
import { StackSettings } from "../../x/automation/stackSettings";


describe("StackSettings", () => {
    it("parses JSON", () => {
        const testStack = {
            secretsProvider: "abc",
            config: {
                plain: "plain",
                secure: { 
                    secure: "secret"
                },
            },
        };
        const result = StackSettings.fromJSON(testStack)
        assert.equal(result.secretsProvider, "abc");
        assert.equal(result.config!["plain"].value, "plain");
        assert.equal(result.config!["secure"].secure, "secret");

    })

    it("JSON serializes", () => {
        const testStack = {
            secretsProvider: "abc",
            config: {
                plain: "plain",
                secure: { 
                    secure: "secret"
                },
            },
        };
        const result = StackSettings.fromJSON(testStack)
        const expected = `{"secretsProvider":"abc","config":{"plain":"plain","secure":{"secure":"secret"}}}`;
        assert.equal(JSON.stringify(result), expected)
    })

    it("parses YAML", () => {
        const stackYaml = `secretsProvider: abc\nconfig:\n  plain: plain\n  secure:\n    secure: secret\n`;
        const result = StackSettings.fromYAML(stackYaml)
        assert.equal(result.secretsProvider, "abc");
        assert.equal(result.config!["plain"].value, "plain");
        assert.equal(result.config!["secure"].secure, "secret");
    })

    it("YAML serializes", () => {
        const testStack = {
            secretsProvider: "abc",
            config: {
                plain: "plain",
                secure: { 
                    secure: "secret"
                },
            },
        };
        const result = StackSettings.fromJSON(testStack).toYAML();
        const expected = `secretsProvider: abc\nconfig:\n  plain: plain\n  secure:\n    secure: secret\n`;
        assert.equal(result, expected);
    })
})