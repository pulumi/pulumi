// Copyright 2025-2025, Pulumi Corporation.
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

import * as assert from "node:assert";
import * as path from "node:path";
import * as execa from "execa";
import { Analyzer } from "../../../provider/experimental/analyzer";

describe("Analyzer", function () {
    before(() => {
        // We need to link in the pulumi package to the testdata directories so
        // that the analyzer can find it and determine pulumi types like
        // ComponentResource or Output.
        // We have a .yarnrc at the repo root that sets a mutex to prevent
        // concurrent yarn installs. This avoids issues in integration tests.
        // However, for these tests we want to run inside yarn, which causes a
        // deadlock. Passing --no-default-rc makes yarn ignore the .yarnrc.
        // There are no issues here with concurrent yarn runs.
        const dir = path.join(__dirname, "testdata");
        execa.sync("yarn", ["install", "--no-default-rc", "--non-interactive"], { cwd: dir });
        execa.sync("yarn", ["link", "@pulumi/pulumi", "--no-default-rc", "--non-interactive"], { cwd: dir });
    });

    it("infers simple types", async function () {
        const dir = path.join(__dirname, "testdata", "simple-types");
        const analyzer = new Analyzer(dir);
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    aNumber: { type: "number", plain: true },
                    aString: { type: "string", plain: true },
                    aBoolean: { type: "boolean", plain: true },
                },
                outputs: {
                    outNumber: { type: "number" },
                    outString: { type: "string" },
                    outBoolean: { type: "boolean" },
                },
            },
        });
    });

    it("infers optional types", async function () {
        const dir = path.join(__dirname, "testdata", "optional-types");
        const analyzer = new Analyzer(dir);
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    optionalNumber: { type: "number", optional: true, plain: true },
                },
                outputs: {
                    optionalOutputNumber: { type: "number", optional: true },
                },
            },
        });
    });

    it("infers input types", async function () {
        const dir = path.join(__dirname, "testdata", "input-types");
        const analyzer = new Analyzer(dir);
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    aNumber: { type: "number" },
                    anOptionalString: { type: "string", optional: true },
                },
                outputs: {},
            },
        });
    });

    it("rejects bad args", async function () {
        const dir = path.join(__dirname, "testdata", "bad-args");
        const analyzer = new Analyzer(dir);
        assert.throws(
            () => analyzer.analyze(),
            /Error: Component 'MyComponent' constructor 'args' parameter must be an interface/,
        );
    });
});
