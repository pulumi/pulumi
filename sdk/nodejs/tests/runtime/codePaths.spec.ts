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

import * as assert from "assert";
import { packageArchivePath } from "../../runtime/closure/codePaths";

describe("packageArchivePath", () => {
    it("keeps plain node_modules paths as-is", () => {
        assert.strictEqual(packageArchivePath("node_modules/semver", "semver"), "node_modules/semver");
    });

    it("keeps nested node_modules paths as-is", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/execa/node_modules/semver", "semver"),
            "node_modules/execa/node_modules/semver",
        );
    });

    it("keeps scoped package paths as-is", () => {
        assert.strictEqual(packageArchivePath("node_modules/@scope/name", "@scope/name"), "node_modules/@scope/name");
    });

    it("keeps workspace paths as-is", () => {
        assert.strictEqual(packageArchivePath("../node_modules/semver", "semver"), "../node_modules/semver");
    });

    it("flattens pnpm store paths", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/.pnpm/semver@7.5.4/node_modules/semver", "semver"),
            "node_modules/semver",
        );
    });

    it("flattens scoped pnpm store paths", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/.pnpm/@scope+name@1.0.0/node_modules/@scope/name", "@scope/name"),
            "node_modules/@scope/name",
        );
    });

    it("flattens peer-suffixed pnpm store paths", () => {
        assert.strictEqual(
            packageArchivePath("node_modules/.pnpm/foo@1.0.0_react@18.2.0/node_modules/foo", "foo"),
            "node_modules/foo",
        );
    });

    it("only matches .pnpm as a full path segment", () => {
        assert.strictEqual(packageArchivePath("node_modules/.pnpm-utils", ".pnpm-utils"), "node_modules/.pnpm-utils");
    });
});
