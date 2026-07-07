// Copyright 2024, Pulumi Corporation.
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
import * as os from "os";
import * as path from "path";
import * as process from "process";
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
            "@types/mocha": "^10.0.0",
            "@types/node": nodeTypesVersion,
            "@types/semver": "^7.5.6",
            execa: "^5.1.0",
            mocha: "^11.0.0",
            "mocha-suppress-logs": "^0.5.1",
            mockpackage: "file:mockpackage-src",
            semver: "^7.5.4",
            "ts-node": "^7.0.1",
            typescript: typescriptVersion,
        },
        overrides: {
            typescript: typescriptVersion,
        },
        pnpm: {
            overrides: {
                typescript: typescriptVersion,
            },
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
        const srcPath = path.join(entry.parentPath ?? entry.path, entry.name);
        const destPath = srcPath.replace(src, dest);
        const destDir = path.dirname(destPath);

        if (entry.isFile()) {
            await fs.mkdir(destDir, { recursive: true });
            await fs.copyFile(srcPath, destPath);
        }
    }
}

// The package managers we run the closure tests against. npm and pnpm lay out
// node_modules differently (pnpm uses a symlinked virtual store under
// node_modules/.pnpm), so we exercise both to ensure function serialization
// resolves module names correctly regardless of the on-disk layout.
type PackageManager = "npm" | "pnpm";

async function install(packageManager: PackageManager, dir: string) {
    if (packageManager === "pnpm") {
        // Force the isolated node-linker so we actually exercise the
        // node_modules/.pnpm/<pkg>@<version>/node_modules/<pkg> layout, even if
        // the environment defaults to a hoisted layout.
        await fs.writeFile(path.join(dir, ".npmrc"), "node-linker=isolated\n");
        // Use pnpm 10, we don't support 11 yet https://github.com/pulumi/pulumi/issues/22893
        await execa("corepack", ["use", "pnpm@^10.0.0"], {
            cwd: dir,
            env: { COREPACK_ENABLE_DOWNLOAD_PROMPT: "0" },
        });
    } else {
        await execa("npm", ["install", "--install-links"], { cwd: dir });
    }
}

async function run(packageManager: PackageManager, typescriptVersion: string, nodeTypesVersion: string) {
    const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), "closure-test-"));
    const sdkRoot = path.join(__dirname, "..", "..", "..");
    const sdkRootBin = path.join(sdkRoot, "bin");
    // Add a random suffix to the package name to avoid any issues with npm caching the tgz.
    const packageName = `pulumi-${randomInt(10000, 99999)}.tgz`;
    const pulumiPackagePath = path.join(tmpDir, packageName);
    await pack(sdkRootBin, pulumiPackagePath);
    await writePackageJSON(tmpDir, pulumiPackagePath, typescriptVersion, nodeTypesVersion);
    await copyDir(path.join(sdkRoot, "tests", "runtime", "testdata", "closure-tests"), tmpDir);

    await install(packageManager, tmpDir);

    await execa("npx", ["--no-install", "tsc"], { cwd: tmpDir });

    await execa("npx", ["--no-install", "mocha", "--timeout", "30000", "test.js"], {
        cwd: tmpDir,
        stdio: "inherit",
        env: { CLOSURE_TEST_PACKAGE_MANAGER: packageManager, COREPACK_ENABLE_DOWNLOAD_PROMPT: "0" },
    });

    await fs.rm(tmpDir, { recursive: true, force: true });
}

async function main() {
    for (const [ts, typesNode] of [
        ["~3.8.3", "ts3.8"], // Latest 3.8.x, this is the default version.
        ["<4.8.0", "ts4.7"], // Before 4.8.0, the typescript API we use has some breaking changes in 4.8.0.
        ["^4.9.5", "ts4.9"], // Latest 4.x.x
        ["<5.2.0", "ts5.1"], // Awaiter changed slightly in 5.2.0 https://github.com/microsoft/TypeScript/pull/56296
        ["^5.2.0", "latest"], // Latest 5.x.x
    ]) {
        await run("npm", ts, typesNode);
    }

    // Also run the suite under pnpm for the default TypeScript version. The module-resolution logic that pnpm exercises
    // is independent of the TypeScript version, so a single run is enough to guard against regressions in pnpm's
    // symlinked node_modules layout.
    await run("pnpm", "~3.8.3", "ts3.8");
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
