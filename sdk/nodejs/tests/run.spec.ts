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
// TODO(@Robbie): Question: how do I import a function for a unit test
//                without exporting that function?
import { hasPkgDeclared } from "../cmd/run/pkg";

describe("hasPkgDeclared", () => {
    const pkgs: Record<string, any> = {
        'dependencies': {
            "myDep": "3.4.0",
            "rightPad": "9.0.0",
        },
    };
    it("finds packages declared in a record", () => {
        assert.strictEqual(hasPkgDeclared("myDep", pkgs), true);
        assert.strictEqual(hasPkgDeclared("rightPad", pkgs), true);
    });

    it("doesn't find non-existant packages", () => {
        assert.strictEqual(hasPkgDeclared("fooman", pkgs), false);
        assert.strictEqual(hasPkgDeclared("barman", pkgs), false);
    });
});
