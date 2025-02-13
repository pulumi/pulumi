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

import * as assert from "assert";
import { ComponentProvider } from "../../../provider/experimental/provider";

describe("validateResourceType", function () {
    it("throws", function () {
        for (const resourceType of ["not-valid", "not:valid", "pkg:not-valid-module:type:", "pkg:index"]) {
            try {
                ComponentProvider.validateResourceType("a", resourceType);
                assert.fail("expected error");
            } catch (err) {
                // pass
            }
        }
    });

    it("accepts", function () {
        for (const resourceType of ["pkg:index:type", "pkg::type", "pkg:index:Type123"]) {
            ComponentProvider.validateResourceType("pkg", resourceType);
        }
    });
});
