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
import { ProjectSettings } from "../../x/automation/projectSettings";


describe("ProjectSettings", () => {
    it("parses full json project definition", () => {
        const testProj = {
            name: "foo",
            runtime: {
                name: "nodejs",
                options: { key: "value" },
            },
        };
        const result = ProjectSettings.fromJSON(testProj)
        assert.equal(result.name, "foo");
        assert.equal(result.runtime.name, "nodejs");
        assert.equal(result.runtime.options!["key"], "value");

    })
    it("parses simplified ProjectRuntimeInfo", () => {
        const testProj = {
            name: "foo",
            runtime: "nodejs",
        };
        const result = ProjectSettings.fromJSON(testProj)
        assert.equal(result.name, "foo");
        assert.equal(result.runtime.name, "nodejs");
    })
    it("JSON serializes the simplified form ProjectRuntimeInfo.info", () => {
        const testProj = {
            name: "foo",
            runtime: "nodejs",
        };
        const result = ProjectSettings.fromJSON(testProj)
        assert.equal(JSON.stringify(result), `{"name":"foo","runtime":"nodejs"}`)
    })
    it("JSON serializes the complex form ProjectRuntimeInfo.info", () => {
        const testProj = {
            name: "foo",
            runtime: {
                name: "nodejs",
                options: { key: "val" },
            },
        };
        const result = ProjectSettings.fromJSON(testProj);
        const expected = `{"name":"foo","runtime":{"name":"nodejs","options":{"key":"val"}}}`;
        assert.equal(JSON.stringify(result), expected);
    })
    it("parses full YAML project definition", () => {
        const projectYaml = `name: foo\nruntime:\n  name: nodejs\n  options:\n    key: val\n`;
        const result = ProjectSettings.fromYAML(projectYaml);
        assert.equal(result.name, "foo");
        assert.equal(result.runtime.name, "nodejs");
        assert.equal(result.runtime.options!["key"], "val");

    })
    it("parses simplified YALM ProjectRuntimeInfo", () => {
        const projectYaml = `name: foo\nruntime: nodejs\n`;
        const result = ProjectSettings.fromYAML(projectYaml)
        assert.equal(result.name, "foo");
        assert.equal(result.runtime.name, "nodejs");
    })
    it("YAML serializes the simplified form ProjectRuntimeInfo.info", () => {
        const testProj = {
            name: "foo",
            runtime: "nodejs",
        };
        const result = ProjectSettings.fromJSON(testProj).toYAML();
        assert.equal(result, `name: foo\nruntime: nodejs\n`)
    })
    it("YAML serializes the complex form ProjectRuntimeInfo.info", () => {
        const testProj = {
            name: "foo",
            runtime: {
                name: "nodejs",
                options: { key: "val" },
            },
        };
        const result = ProjectSettings.fromJSON(testProj).toYAML();
        const expected = `name: foo\nruntime:\n  name: nodejs\n  options:\n    key: val\n`;
        assert.equal(result, expected);
    })
})