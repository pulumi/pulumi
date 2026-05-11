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

import * as fs from "fs";
import * as yaml from "js-yaml";
import * as path from "path";

/**
 * Filenames recognized as Node.js package manifests, in priority order. pnpm allows `package.yaml` as an alternative to
 * `package.json` (see https://pnpm.io/package_json), and prefers `package.json` if both exist.
 *
 * @internal
 */
export const PACKAGE_MANIFEST_NAMES = ["package.json", "package.yaml"];

/**
 * Reads and parses the package manifest from `dir`. If both `package.json` and `package.yaml` exist, `package.json` is
 * preferred. Throws if neither file exists or if the file exists but cannot be read or parsed.
 *
 * @internal
 */
export function readPackageManifest(dir: string): { data: Record<string, any>; path: string } {
    for (const name of PACKAGE_MANIFEST_NAMES) {
        const p = path.join(dir, name);
        let content: string;
        try {
            content = fs.readFileSync(p, { encoding: "utf-8" });
        } catch (err) {
            if ((err as NodeJS.ErrnoException).code === "ENOENT") {
                continue;
            }
            throw err;
        }
        try {
            const data = parseManifestContent(name, content);
            return { data, path: p };
        } catch (err) {
            throw new Error(`could not parse ${p}: ${(err as Error).message}`);
        }
    }
    throw new Error(`no package.json or package.yaml in ${dir}`);
}

/**
 * Walks up from `startDir` looking for the nearest directory containing a `package.json` or `package.yaml`. Returns the
 * path of the manifest file, or `undefined` if no manifest is found anywhere up the tree. If both files exist in the
 * same directory, `package.json` is preferred.
 *
 * @internal
 */
export function searchupPackageManifest(startDir: string): string | undefined {
    let dir = startDir;
    while (true) {
        for (const name of PACKAGE_MANIFEST_NAMES) {
            const p = path.join(dir, name);
            if (fs.existsSync(p)) {
                return p;
            }
        }
        const parent = path.dirname(dir);
        if (parent === dir) {
            return undefined;
        }
        dir = parent;
    }
}

function parseManifestContent(name: string, content: string): Record<string, any> {
    if (name.endsWith(".json")) {
        return JSON.parse(content);
    }
    return (yaml.safeLoad(content) as Record<string, any>) || {};
}
