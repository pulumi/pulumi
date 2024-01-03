// Copyright 2016-2020, Pulumi Corporation.
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
import * as fs from "fs";
import got from "got";
import * as os from "os";
import * as path from "path";
import * as semver from "semver";
import * as tmp from "tmp";
import { version as DEFAULT_VERSION } from "../version";
import { minimumVersion } from "./minimumVersion";
import { createCommandError } from "./errors";

const SKIP_VERSION_CHECK_VAR = "PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK";

/** @internal */
export class CommandResult {
    stdout: string;
    stderr: string;
    code: number;
    err?: Error;
    constructor(stdout: string, stderr: string, code: number, err?: Error) {
        this.stdout = stdout;
        this.stderr = stderr;
        this.code = code;
        this.err = err;
    }
    toString(): string {
        let errStr = "";
        if (this.err) {
            errStr = this.err.toString();
        }
        return `code: ${this.code}\n stdout: ${this.stdout}\n stderr: ${this.stderr}\n err?: ${errStr}\n`;
    }
}

export interface PulumiOptions {
    version?: semver.SemVer;
    root?: string;
    /**
     * Skips the minimum CLI version check, see `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK`.
     * @internal
     */
    skipVersionCheck?: boolean;
}

export class Pulumi {
    private constructor(readonly command: string, readonly version: semver.SemVer | null, readonly root?: string) {}

    /**
     * Get a new Pulumi instance that uses the installation in `opts.root`.
     * Defaults to using the pulumi binary found in $PATH if no installation
     * root is specified.  If `opts.version` is specified, it validates that
     * the CLI is compatible with the requested version and throws an error if
     * not.
     */
    static async get(opts?: PulumiOptions): Promise<Pulumi> {
        const command = opts?.root ? path.resolve(path.join(opts.root, "bin/pulumi")) : "pulumi";
        const { stdout } = await exec(command, ["version"]);
        const skipVersionCheck = opts?.skipVersionCheck !== undefined ? opts.skipVersionCheck : false;
        const version = parseAndValidatePulumiVersion(minimumVersion, stdout.trim(), skipVersionCheck);
        return new Pulumi(command, version, opts?.root);
    }

    /**
     * Installs the Pulumi CLI.
     *
     * @param opts.version the version of the CLI. Defaults to the CLI version matching the SDK version.
     * @param opts.root the directory to install the CLI in. Defaults to $HOME/.pulumi/versions/$VERSION.
     */
    static async install(opts?: PulumiOptions): Promise<Pulumi> {
        const optsWithDefaults = withDefaults(opts);
        try {
            return await Pulumi.get(optsWithDefaults);
        } catch (err) {
            // ignore
        }

        if (process.platform === "win32") {
            await Pulumi.installWindows(optsWithDefaults);
        } else {
            await Pulumi.installPosix(optsWithDefaults);
        }

        return await Pulumi.get(optsWithDefaults);
    }

    private static async installWindows(opts: Required<PulumiOptions>): Promise<void> {
        //TODO: const response = await got("https://get.pulumi.com/install.ps1");
        const response = await got("get.pulumi-staging.io/install.ps1");
        const script = await writeTempFile(response.body);

        try {
            const command = process.env.SystemRoot
                ? path.join(process.env.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
                : "powershell.exe";

            const args = [
                "-NoProfile",
                "-InputFormat",
                "None",
                "-ExecutionPolicy",
                "Bypass",
                "-File",
                script.path,
                "-NoEditPath",
                "-InstallRoot",
                opts.root,
                "-Version",
                `${opts.version}`,
            ];

            await exec(command, args);
        } finally {
            script.cleanup();
        }
    }

    private static async installPosix(opts: Required<PulumiOptions>): Promise<void> {
        // TODO: const response = await got("https://get.pulumi.com/install.sh");
        const response = await got("https://get.pulumi-staging.io/install.sh");
        const script = await writeTempFile(response.body);

        try {
            const args = [script.path, "--no-edit-path", "--install-root", opts.root, "--version", `${opts.version}`];

            await exec("/bin/sh", args);
        } finally {
            script.cleanup();
        }
    }

    /** @internal */
    public run(
        args: string[],
        cwd: string,
        additionalEnv: { [key: string]: string },
        onOutput?: (data: string) => void,
    ): Promise<CommandResult> {
        // all commands should be run in non-interactive mode.
        // this causes commands to fail rather than prompting for input (and thus hanging indefinitely)

        if (!args.includes("--non-interactive")) {
            args.push("--non-interactive");
        }

        // Prepend the installation root to the path to ensure we pickup the
        // matching bundled plugins.
        if (this.root) {
            additionalEnv["PATH"] = prependRootToPath(this.root, additionalEnv["PATH"] || process.env["PATH"]);
        }

        return exec(this.command || "pulumi", args, cwd, additionalEnv, onOutput);
    }
}

async function exec(
    command: string,
    args: string[],
    cwd?: string,
    additionalEnv?: { [key: string]: string },
    onOutput?: (data: string) => void,
): Promise<CommandResult> {
    const unknownErrCode = -2;

    const env = additionalEnv ? { ...additionalEnv } : undefined;

    try {
        const proc = execa(command, args, { env, cwd });

        if (onOutput && proc.stdout) {
            proc.stdout!.on("data", (data: any) => {
                if (data?.toString) {
                    data = data.toString();
                }
                onOutput(data);
            });
        }

        const { stdout, stderr, exitCode } = await proc;
        const commandResult = new CommandResult(stdout, stderr, exitCode);
        if (exitCode !== 0) {
            throw createCommandError(commandResult);
        }

        return commandResult;
    } catch (err) {
        const error = err as Error;
        throw createCommandError(new CommandResult("", error.message, unknownErrCode, error));
    }
}

function withDefaults(opts?: PulumiOptions): Required<PulumiOptions> {
    let version = opts?.version;
    if (!version) {
        version = new semver.SemVer(DEFAULT_VERSION);
    }
    let root = opts?.root;
    if (!root) {
        root = path.join(os.homedir(), ".pulumi", "versions", `${version}`);
    }
    const skipVersionCheck = opts?.skipVersionCheck !== undefined ? opts.skipVersionCheck : false;
    return { version, root, skipVersionCheck };
}

function writeTempFile(contents: string): Promise<{ path: string; cleanup: () => void }> {
    return new Promise<{ path: string; cleanup: () => void }>((resolve, reject) => {
        tmp.file((tmpErr, tmpPath, fd, cleanup) => {
            if (tmpErr) {
                reject(tmpErr);
            } else {
                fs.writeFile(fd, contents, (writeErr) => {
                    if (writeErr) {
                        cleanup();
                        reject(writeErr);
                    } else {
                        resolve({ path: tmpPath, cleanup });
                    }
                });
            }
        });
    });
}

function prependRootToPath(root: string, base?: string): string {
    const pulumiBin = path.join(root, "bin");
    if (!base) {
        return pulumiBin;
    }
    const sep = os.platform() === "win32" ? ";" : ":";
    return pulumiBin + sep + base;
}

/**
 * @internal
 * Throws an error if the Pulumi CLI version is not valid.
 *
 * @param minVersion The minimum acceptable version of the Pulumi CLI.
 * @param currentVersion The currently known version. `null` indicates that the current version is unknown.
 * @param optOut If the user has opted out of the version check.
 */
export function parseAndValidatePulumiVersion(
    minVersion: semver.SemVer,
    currentVersion: string,
    optOut: boolean,
): semver.SemVer | null {
    const version = semver.parse(currentVersion);
    if (optOut) {
        return version;
    }
    if (version == null) {
        throw new Error(
            `Failed to parse Pulumi CLI version. This is probably an internal error. You can override this by setting "${SKIP_VERSION_CHECK_VAR}" to "true".`,
        );
    }
    if (minVersion.major < version.major) {
        throw new Error(
            `Major version mismatch. You are using Pulumi CLI version ${currentVersion} with Automation SDK v${minVersion.major}. Please update the SDK.`,
        );
    }
    if (minVersion.compare(version) === 1) {
        throw new Error(
            `Minimum version requirement failed. The minimum CLI version requirement is ${minVersion.toString()}, your current CLI version is ${currentVersion}. Please update the Pulumi CLI.`,
        );
    }
    return version;
}
