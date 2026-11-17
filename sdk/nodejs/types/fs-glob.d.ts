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

// `fs.promises.glob` was added in Node.js 22, but we type-check against `@types/node@14`, which does not declare it.
// This type declaration can be removed once `@types/node` is bumped to >= 22.

declare module "fs/promises" {
    function glob(pattern: string | string[], options?: { cwd?: string }): AsyncIterableIterator<string>;
}
