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
//

import * as runtime from "@pulumi/pulumi/runtime" // @pulumi dependency is not included
import * as semver from "semver" // npm dependency
import * as myRandom from "my-random" // workspace dependency
import * as myDynamicProvider from "my-dynamic-provider" // workspace dependency

const random = new myRandom.MyRandom("plop", {});

export const id = random.randomID;

export const version = semver.parse("1.2.3");

const dynamicProviderResource = new myDynamicProvider.MyDynamicProviderResource("prov", {});

(async function () {
  const deps = await runtime.computeCodePaths();

  const actual = JSON.stringify([...deps.keys()].sort());
  // semver depends on lru-cache (6.0.0), which depends on yallist
  const expected = `[` +
    `"../node_modules/is-finite",` +
    `"../node_modules/lru-cache",` +
    `"../node_modules/my-dynamic-provider",` +
    `"../node_modules/my-random",` +
    `"../node_modules/semver",` +
    `"../node_modules/yallist"` +
    `]`;

  if (actual !== expected) {
    throw new Error(`Got '${actual}' expected '${expected}'`)
  }
})();