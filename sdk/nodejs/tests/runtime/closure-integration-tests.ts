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

import { randomInt } from "crypto";
import execa from "execa";
import * as fs from "fs/promises";
import * as path from "path";
import * as process from "process";
import * as tmp from "tmp";
import { pack } from "./pack";

// Write a package.json that installs the local pulumi package and ensures we
// have the right typescript version.
async function writePackageJSON(
    dir: string,
    pulumiPackagePath: string,
    typescriptVersion: string,
    nodeTypesVersion: string,
) {
    const packageJSON = {
        name: "closure-tests",
        version: "1.0.0",
        license: "Apache-2.0",
        dependencies: {
            "@pulumi/pulumi": pulumiPackagePath,
            "@types/mocha": "^9.0.0",
            "@types/node": nodeTypesVersion,
            "@types/semver": "^7.5.6",
            mocha: "^9.0.0",
            "mocha-suppress-logs": "^0.5.1",
            mockpackage: "file:mockpackage-src",
            semver: "^7.5.4",
            "ts-node": "^7.0.1",
            typescript: typescriptVersion,
        },
        resolutions: {
            typescript: typescriptVersion,
        },
    };

    await fs.writeFile(path.join(dir, "package.json"), JSON.stringify(packageJSON, undefined, 4));
}

async function copyDir(src: string, dest: string) {
    const entries: any[] = await fs.readdir(
        src,
        { recursive: true, withFileTypes: true } as any /* recursive option is new in node 18 */,
    );

    for (const entry of entries) {
        const srcPath = path.join(entry.path, entry.name);
        const destPath = srcPath.replace(src, dest);
        const destDir = path.dirname(destPath);

        if (entry.isFile()) {
            await fs.mkdir(destDir, { recursive: true });
            await fs.copyFile(srcPath, destPath);
        }
    }
}

async function run(typescriptVersion: string, nodeTypesVersion: string) {
    const tmpDir = tmp.dirSync({ prefix: "closure-test-", unsafeCleanup: true });
    const sdkRoot = path.join(__dirname, "..", "..", "..");
    const sdkRootBin = path.join(sdkRoot, "bin");
    // Add a random suffix to the package name to avoid any issues with yarn caching the tgz.
    const packageName = `pulumi-${randomInt(10000, 99999)}.tgz`;
    const pulumiPackagePath = path.join(tmpDir.name, packageName);
    await pack(sdkRootBin, pulumiPackagePath);
    await writePackageJSON(tmpDir.name, pulumiPackagePath, typescriptVersion, nodeTypesVersion);
    await copyDir(path.join(sdkRoot, "tests", "runtime", "testdata", "closure-tests"), tmpDir.name);

    await execa("yarn", ["install"], { cwd: tmpDir.name });

    await execa("yarn", ["tsc"], { cwd: tmpDir.name });

    await execa("yarn", ["mocha", "--timeout", "30000", "test.js"], {
        cwd: tmpDir.name,
        stdio: "inherit",
    });

    tmpDir.removeCallback();
}

async function main() {
    for (const [ts, typesNode] of [
        ["~3.8.3", "ts3.8"], // Latest 3.8.x, this is the default version.
        ["<4.8.0", "ts4.7"], // Before 4.8.0, the typescript API we use has some breaking changes in 4.8.0.
        ["^4.9.5", "ts4.9"], // Latest 4.x.x
        ["<5.2.0", "ts5.1"], // Awaiter changed slightly in 5.2.0 https://github.com/microsoft/TypeScript/pull/56296
        ["^5.2.0", "latest"], // Latest 5.x.x
    ]) {
        await run(ts, typesNode);
    }
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
