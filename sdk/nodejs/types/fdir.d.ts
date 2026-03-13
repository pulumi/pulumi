// Copyright 2026-2026, Pulumi Corporation.
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

// fdir 6.5 ships its own types, but it uses `index.d.cts` and `index.d.mts` for its type declaration files. We use
// TypeScript 3.8.3, which does not know how to resolve cts/mts files. This file provides minimal type declarations for
// our use of fdir. The reason why we need fdir 6.5 in the first place is because we need a recent version of
// @npmcli/arborist so that we pull in a recent version of tar. Older versions of tar have security vulnerabilities that
// are not backported.

declare module "fdir" {
    class fdir {
        withBasePath(): fdir;
        glob(pattern: string): fdir;
        crawl(directory: string): {
            withPromise(): Promise<string[]>;
        };
    }

    export { fdir };
}
