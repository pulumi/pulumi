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

import execa from "execa";
import * as fs from "fs/promises";
import * as path from "path";

/** @internal */
export async function pack(dir: string, out: string) {
    const { stdout } = await execa("npm", ["pack"], { cwd: dir });
    const tarball = stdout.trim();
    const tarballPath = path.join(dir, tarball);
    try {
        await fs.rename(tarballPath, out);
    } catch (error) {
        // Rename can fail if we move across devices on Windows, fallback to
        // copying and removing.
        await fs.copyFile(tarballPath, out);
        await fs.unlink(tarballPath);
    }
}
