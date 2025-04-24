// Copyright 2024-2024, Pulumi Corporation.
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

import * as runtime from "@pulumi/pulumi/runtime"

(async function () {
    const deps = await runtime.computeCodePaths() as Map<string, string>;
    // Deps might include more than just the direct dependencies, but the
    // precise results depend on how the packages are hoisted within
    // node_modules. This can change based on the version of the package
    // manager and the dependencies of @pulumi/pulumi.
    // 
    // For example for this nesting:
    //
    // node_modules
    // └─┬ semver
    //   └── lru-cache
    //
    // deps only includes `node_modules/semver`. However if they are siblings,
    // it will include both `node_modules/semver` and `node_modules/lru-cache`.
    //
    // We only assert that the direct dependencies are included, which are
    // guaranteed to be stable.
    const directDependencies = [`node_modules/semver`]

    const depPaths = [...deps.keys()]
    for (const expected of directDependencies) {
        const depPath = depPaths.find((path) => path.includes(expected));
        if (!depPath) {
            throw new Error(`Expected to find a path matching ${expected}, got ${depPaths}`)
        }
    }
})();