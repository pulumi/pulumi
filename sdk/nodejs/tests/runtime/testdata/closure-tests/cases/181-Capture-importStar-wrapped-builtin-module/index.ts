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

export const description = "Capture __importStar wrapped builtin module";

// Simulate the wrapper that TypeScript emits for `import * as crypto from "crypto"` when compiling with `module:
// "nodenext"`. The __importStar helper wraps the require() result in a new object with getter properties and a
// `default` property holding the original module.
function __importStar(mod: any): any {
    if (mod && mod.__esModule) return mod;
    const result: any = {};
    if (mod != null) {
        for (const k of Object.getOwnPropertyNames(mod)) {
            if (k !== "default") {
                Object.defineProperty(result, k, { enumerable: true, get: () => mod[k] });
            }
        }
    }
    Object.defineProperty(result, "default", { enumerable: true, value: mod });
    return result;
}

const crypto = __importStar(require("crypto"));

export const func = () => crypto;
