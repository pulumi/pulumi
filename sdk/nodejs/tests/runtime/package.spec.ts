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

type Exported = string | {
    import?: string;
    require?: string;
}
function assertElementIn<T>(element: T, collection: T[]) {
    return assert.strictEqual(true, collection.includes(element))
}

describe("module", () => {
    it("remaps exports correctly for mockpackage", () => {
        assert.strictEqual(pkg.getModuleFromPath("mockpackage/lib/index.js"), "mockpackage")
    });
    it("should return undefined on unexported members", () => {
        assert.strictEqual(pkg.getModuleFromPath("mockpackage/lib/external.js"), undefined)
    });
})
describe("To exclude private subfolders from patterns, null targets can be used", () => {
        // ./node_modules/es-module-package/package.json
        const packagedef = {
            "name": "es-module-package",
            "exports": {
                "./features/*": "./src/features/*.js",
                "./features/private-internal/*": null
            }
        }
        it(`hides null(blacklisted) wildcard paths`, () => {
            //import featureInternal from 'es-module-package/features/private-internal/m';
            // Throws: ERR_PACKAGE_PATH_NOT_EXPORTED
            assert.strictEqual(pkg.getModuleFromPath('es-module-package/features/private-internal/m', packagedef), undefined);
        })
        it(`handles wildcard paths`, () => {
            //import featureX from 'es-module-package/features/x';
            // Loads ./node_modules/es-module-package/src/features/x.js
            assert.strictEqual(pkg.getModuleFromPath('es-module-package/src/features/x.js', packagedef), 'es-module-package/features/x')
        })
});
describe("basic package exports", () => {
    // https://nodejs.org/api/packages.html#package-entry-points
    const packagedef = {
        "name": "my-mod",
        "exports": {
            ".": "./lib/index.js",
            "./lib": "./lib/index.js",
            "./lib/index": "./lib/index.js",
            "./lib/index.js": "./lib/index.js",
            "./feature": "./feature/index.js",
            "./feature/index.js": "./feature/index.js",
            "./package.json": "./package.json"
        }
    }
    it("handles multiple aliases 1", () => {
        assert.strictEqual(pkg.getModuleFromPath("my-mod/lib/index.js", packagedef), "my-mod/lib/index.js"); // highest specificity
            // not "my-mod",
            // not "my-mod/lib",
            // not "my-mod/lib/index",
    })
    it("handles multiple aliases 2", () => {
        assert.strictEqual(pkg.getModuleFromPath("my-mod/feature/index.js", packagedef), "my-mod/feature/index.js"); // highest specificity
            // not "my-mod/feature",
    })
    it("returns with no modification", () => {
        assert.strictEqual(pkg.getModuleFromPath("my-mod/package.json", packagedef), "my-mod/package.json")
    })
});
describe("wildcard package exports", () => {
    const packagedef = {
        "name": "my-mod",
        "exports": {
            ".": "./lib/index.js",
            "./lib": "./lib/index.js",
            "./lib/*": "./lib/*.js",
            "./feature": "./feature/index.js",
            "./feature/*": "./feature/*.js",
            "./package.json": "./package.json"
        }
    }
    it("wildcard module", () => {
        assert.strictEqual(pkg.getModuleFromPath("my-mod/lib/foobar.js", packagedef), "my-mod/lib/foobar")
        assert.strictEqual(pkg.getModuleFromPath("my-mod/lib/foo", packagedef), "my-mod/lib/foo")
        assert.strictEqual(pkg.getModuleFromPath("my-mod/feature/foobar.js", packagedef), "my-mod/feature/foobar")
    })
});
describe("conditional import/require package exports", () => {
    const packagedef = {
        // package.json
        "name": "that-mod",
        "exports": {
            ".": "./main.js",
            "./feature": {
                "node": "./feature-node.js",
                "default": "./feature.js"
            }
        },
        "type": "module"
    }
    it("remaps conditional node/default nested packages", () => {
        assert.strictEqual(pkg.getModuleFromPath("that-mod/main.js", packagedef), "that-mod")
        assert.strictEqual(pkg.getModuleFromPath("that-mod/feature-node.js", packagedef), "that-mod/feature")
        assert.strictEqual(pkg.getModuleFromPath("that-mod/feature.js", packagedef), "that-mod/feature")
    })
});
describe("conditional import/require package exports", () => {
    const packagedef = {
        // package.json
        "name": "this-mod",
        "main": "./main-require.cjs",
        "exports": {
            "import": "./main-module.js",
            "require": "./main-require.cjs"
        },
        "type": "module"
    }
    it("remaps to main pkg", () => {
        assert.strictEqual(pkg.getModuleFromPath("this-mod/main-require.js", packagedef), "this-mod")
        assert.strictEqual(pkg.getModuleFromPath("this-mod/main-require.cjs", packagedef), "this-mod")
    })
});