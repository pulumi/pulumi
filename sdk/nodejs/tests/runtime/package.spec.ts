// Copyright 2016-2018, Pulumi Corporation.
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
import * as pkg from "../../runtime/closure/package";

describe("module", () => {
    it("remaps exports correctly for mockpackage", () => {
        assert.equal(pkg.getModuleFromPath("mockpackage/lib/index.js"), "mockpackage")
    });
    it("should return undefined on unexported members", () => {
        assert.equal(pkg.getModuleFromPath("mockpackage/lib/external.js"), undefined)
    });
});