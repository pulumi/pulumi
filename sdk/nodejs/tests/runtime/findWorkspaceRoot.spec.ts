// Copyright 2016-2022, Pulumi Corporation.
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
import * as path from "path";
import { findWorkspaceRoot } from "../../runtime/closure/codePaths";

const testdata = (...p: string[]) => path.join(__dirname, "..", "..", "..", "tests", "runtime", "testdata", ...p);

describe("findWorkspaceRoot", () => {
    it("finds the root of a workspace", async () => {
        const root = await findWorkspaceRoot(testdata("workspace", "project"));
        assert.notStrictEqual(root, null);
        assert.strictEqual(root, testdata("workspace"));
    });

    it("returns null if we are not in a workspace", async () => {
        const root = await findWorkspaceRoot(testdata("nested", "project"));
        assert.strictEqual(root, null);
    });

    it("finds the root of a workspace when using yarn's extended declaration", async () => {
        const root = await findWorkspaceRoot(testdata("workspace-extended", "project"));
        assert.notStrictEqual(root, null);
        assert.strictEqual(root, testdata("workspace-extended"));
    });

    it("finds the root of a workspace when in a nested directory", async () => {
        const root = await findWorkspaceRoot(testdata("workspace-nested", "project", "dist"));
        assert.notStrictEqual(root, null);
        assert.strictEqual(root, testdata("workspace-nested"));
    });

    it("finds the root of a workspace passing a file as argument", async () => {
        const root = await findWorkspaceRoot(testdata("workspace-nested", "project", "dist", "index.js"));
        assert.notStrictEqual(root, null);
        assert.strictEqual(root, testdata("workspace-nested"));
    });
});
