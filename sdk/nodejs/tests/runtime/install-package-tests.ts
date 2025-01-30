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

// Write a package.json that installs the local pulumi package and optional dependencies.
async function writePackageJSON(
    dir: string,
    pulumiPackagePath: string,
    dependencies: Record<string, string | undefined>,
) {
    const packageJSON = {
        name: "install-package-tests",
        version: "1.0.0",
        license: "Apache-2.0",
        dependencies: {
            "@pulumi/pulumi": pulumiPackagePath,
            ...dependencies,
        },
    };

    await fs.writeFile(path.join(dir, "package.json"), JSON.stringify(packageJSON, undefined, 4));
}

async function writeTSConfig(dir: string) {
    const tsconfigJSON = {
        compilerOptions: {
            strict: true,
            target: "es2016",
            module: "commonjs",
            moduleResolution: "node",
            declaration: true,
            resolveJsonModule: true,
            sourceMap: true,
            stripInternal: true,
            experimentalDecorators: true,
            pretty: true,
            noFallthroughCasesInSwitch: true,
            noImplicitReturns: true,
            forceConsistentCasingInFileNames: true,
            esModuleInterop: true,
        },
        include: ["index.ts"],
    };

    await fs.writeFile(path.join(dir, "tsconfig.json"), JSON.stringify(tsconfigJSON, undefined, 4));
}

// A simple TypeScript Pulumi program to test that we can load and run TypeScript code.
async function writeProgram(dir: string, projectName: string) {
    const indexTS = `import * as pulumi from "@pulumi/pulumi";
pulumi.runtime.serializeFunction(() => 42);
export const test: number = 42;
`;
    await fs.writeFile(path.join(dir, "index.ts"), indexTS);

    const project = `name: ${projectName}
runtime: nodejs
backend:
  url: 'file://~'
`;
    await fs.writeFile(path.join(dir, "Pulumi.yaml"), project);
}

async function exec(command: string, args: string[], options: execa.Options): Promise<string> {
    const message = `$ ${command} ${args.join(" ")}'\n`;
    const result = await execa(command, args, options);
    return message + result.stdout;
}

async function main() {
    const sdkRoot = path.join(__dirname, "..", "..", "..");
    const sdkRootBin = path.join(sdkRoot, "bin");
    const tmpPackageDir = tmp.dirSync({ prefix: "pulumi-package-", unsafeCleanup: true });
    try {
        // Add a random suffix to the package name to avoid any issues with yarn caching the tgz.
        const packageName = `pulumi-${randomInt(10000, 99999)}.tgz`;
        const pulumiPackagePath = path.join(tmpPackageDir.name, packageName);
        await pack(sdkRootBin, pulumiPackagePath);

        const packageManagers = [
            {
                name: "npm",
                // This version doesn't install peer dependencies automatically.
                version: "^6.0.0",
            },
            {
                name: "npm",
                version: "*", // Latest version.
            },
            {
                name: "yarn",
                // This version doesn't install peer dependencies automatically.
                version: "^1.0.0",
            },
            // We don't support yarn >= 2 yet.
            // {
            //     packageManager: "yarn",
            //     version: "*", // Latest version.
            // },
            {
                name: "pnpm",
                version: "*", // Latest version.
            },
        ];

        // Dependencies to add to package.json.
        const dependencies = [
            {
                // No explicit typescript or ts-node versions, use the vendored versions.
                typescript: undefined,
                "ts-node": undefined,
            },
            {
                typescript: "~3.8.3",
                "ts-node": "^7.0.1",
            },
            {
                typescript: "^4.0.0",
                "ts-node": undefined,
            },
            {
                typescript: "^5.0.0",
                "ts-node": undefined,
            },
            {
                typescript: "^5.0.0",
                "ts-node": "^10.0.0",
            },
        ];

        for (const pm of packageManagers) {
            for (const deps of dependencies) {
                const tmpDir = tmp.dirSync({ prefix: "install-test-", unsafeCleanup: true });
                try {
                    await runTest(tmpDir, pulumiPackagePath, pm.name, pm.version, deps);
                } finally {
                    tmpDir.removeCallback();
                }
            }
        }
    } finally {
        tmpPackageDir.removeCallback();
    }
}

async function runTest(
    tmpDir: tmp.DirResult,
    pulumiPackagePath: string,
    packageManager: string,
    packageManagerVersion: string,
    peerDeps: Record<string, string | undefined>,
) {
    await writePackageJSON(tmpDir.name, pulumiPackagePath, peerDeps);
    await writeTSConfig(tmpDir.name);
    const projectName = `install-test-${packageManager}-${packageManagerVersion}`.replace(/[^a-zA-Z0-9]/g, "-");
    await writeProgram(tmpDir.name, projectName);

    const dependencies = Object.entries(peerDeps)
        .filter(([_, v]) => v !== undefined)
        .map(([p, v]) => `${p}:${v}`)
        .join(", ");
    const dependenciesString = dependencies.length > 0 ? ` with ${dependencies}` : "";

    let logs = "";

    // Get the corepack executable from the yarn bin directory, which allows us
    // to use the version of corepack that's installed as part of our dev
    // dependencies. This avoids having to install corepack globally or in CI.
    const { stdout: bin } = await execa("yarn", ["bin"], {});
    const corepack = path.join(bin.trim(), "corepack");

    // Install the package manager to test.
    logs += await exec(corepack, ["enable"], { cwd: tmpDir.name });
    logs += await exec(corepack, ["use", `${packageManager}@${packageManagerVersion}`], { cwd: tmpDir.name });

    const env = {
        PULUMI_CONFIG_PASSPHRASE: "test",
        PULUMI_HOME: tmpDir.name,
    };

    // Up and down a test stack to ensure we're able to load & run typescript code.
    const stackName = `install-test-${randomInt(10000, 99999)}`;
    try {
        logs += await exec("pulumi", ["stack", "init", stackName], {
            cwd: tmpDir.name,
            env,
        });
        logs += await exec("pulumi", ["up", "--stack", stackName, "--skip-preview"], {
            cwd: tmpDir.name,
            env,
        });

        console.log(`✅ ${packageManager}@${packageManagerVersion}${dependenciesString}`);
    } catch (err) {
        console.log(
            `❌ Failed to run test with ${packageManager}@${packageManagerVersion}${dependenciesString} in ${tmpDir.name}: ${err}`,
        );
        console.log(`Captured stdout: ${logs}`);
        throw err;
    } finally {
        await exec("pulumi", ["destroy", "--stack", stackName, "--yes"], {
            cwd: tmpDir.name,
            env,
        });
        await exec("pulumi", ["stack", "rm", stackName, "--yes"], {
            cwd: tmpDir.name,
            env,
        });
    }
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
