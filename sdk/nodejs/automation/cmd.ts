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

/**
 * @internal
 */
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

export interface PulumiCommandOptions {
    /**
     * The version of the CLI to use. Defaults to the CLI version matching the SDK version.
     */
    version?: semver.SemVer;
    /**
     * The directory to install the CLI in or where to look for an existing
     * installation. Defaults to $HOME/.pulumi/versions/$VERSION.
     */
    root?: string;
    /**
     * Skips the minimum CLI version check, see `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK`.
     */
    skipVersionCheck?: boolean;
}

export class PulumiCommand {
    private constructor(
        readonly command: string,
        readonly version: semver.SemVer | null,
    ) {}

    /**
     * Get a new Pulumi instance that uses the installation in `opts.root`.
     * Defaults to using the pulumi binary found in $PATH if no installation
     * root is specified.  If `opts.version` is specified, it validates that
     * the CLI is compatible with the requested version and throws an error if
     * not. This validation can be skipped by setting the environment variable
     * `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK` or setting
     * `opts.skipVersionCheck` to `true`. Note that the environment variable
     * always takes precedence. If it is set it is not possible to re-enable
     * the validation with `opts.skipVersionCheck`.
     */
    static async get(opts?: PulumiCommandOptions): Promise<PulumiCommand> {
        const command = opts?.root ? path.resolve(path.join(opts.root, "bin/pulumi")) : "pulumi";
        const { stdout } = await exec(command, ["version"]);
        const skipVersionCheck = !!opts?.skipVersionCheck || !!process.env[SKIP_VERSION_CHECK_VAR];
        let min = minimumVersion;
        if (opts?.version && semver.gt(opts.version, minimumVersion)) {
            min = opts.version;
        }
        const version = parseAndValidatePulumiVersion(min, stdout.trim(), skipVersionCheck);
        return new PulumiCommand(command, version);
    }

    /**
     * Installs the Pulumi CLI.
     */
    static async install(opts?: PulumiCommandOptions): Promise<PulumiCommand> {
        const optsWithDefaults = withDefaults(opts);
        try {
            return await PulumiCommand.get(optsWithDefaults);
        } catch (err) {
            // ignore
        }

        if (process.platform === "win32") {
            await PulumiCommand.installWindows(optsWithDefaults);
        } else {
            await PulumiCommand.installPosix(optsWithDefaults);
        }

        return await PulumiCommand.get(optsWithDefaults);
    }

    private static async installWindows(opts: Required<PulumiCommandOptions>): Promise<void> {
        const response = await got("https://get.pulumi.com/install.ps1");
        const script = await writeTempFile(response.body, { extension: ".ps1" });

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

    private static async installPosix(opts: Required<PulumiCommandOptions>): Promise<void> {
        const response = await got("https://get.pulumi.com/install.sh");
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
        onError?: (data: string) => void,
        signal?: AbortSignal,
    ): Promise<CommandResult> {
        // all commands should be run in non-interactive mode.
        // this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
        if (!args.includes("--non-interactive")) {
            args.push("--non-interactive");
        }

        const env = { ...additionalEnv };
        // Prepend the folder where the CLI is installed to the path to ensure
        // we pickup the matching bundled plugins.
        if (path.isAbsolute(this.command)) {
            const pulumiBin = path.dirname(this.command);
            const sep = os.platform() === "win32" ? ";" : ":";
            const envPath = pulumiBin + sep + (additionalEnv["PATH"] || process.env.PATH);
            env["PATH"] = envPath;
        }
        env["PULUMI_AUTOMATION_API"] = "true";

        return exec(this.command, args, cwd, env, onOutput, onError, signal);
    }
}

async function exec(
    command: string,
    args: string[],
    cwd?: string,
    additionalEnv?: { [key: string]: string },
    onOutput?: (data: string) => void,
    onError?: (data: string) => void,
    signal?: AbortSignal,
): Promise<CommandResult> {
    const unknownErrCode = -2;

    const env = additionalEnv ? { ...additionalEnv } : undefined;

    try {
        const proc = execa(command, args, { env, cwd });

        if (onError && proc.stderr) {
            proc.stderr!.on("data", (data: any) => {
                if (data?.toString) {
                    data = data.toString();
                }
                onError(data);
            });
        }

        if (onOutput && proc.stdout) {
            proc.stdout!.on("data", (data: any) => {
                if (data?.toString) {
                    data = data.toString();
                }
                onOutput(data);
            });
        }

        if (signal) {
            signal.addEventListener("abort", () => {
                proc.kill("SIGINT", { forceKillAfterTimeout: false });
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

function withDefaults(opts?: PulumiCommandOptions): Required<PulumiCommandOptions> {
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

function writeTempFile(
    contents: string,
    options?: { extension?: string },
): Promise<{ path: string; cleanup: () => void }> {
    return new Promise<{ path: string; cleanup: () => void }>((resolve, reject) => {
        tmp.file(
            {
                // Powershell requires a `.ps1` extension.
                postfix: options?.extension,
                // Powershell won't execute the script if the file descriptor is open.
                discardDescriptor: true,
            },
            (tmpErr, tmpPath, _fd, cleanup) => {
                if (tmpErr) {
                    reject(tmpErr);
                } else {
                    fs.writeFile(tmpPath, contents, (writeErr) => {
                        if (writeErr) {
                            cleanup();
                            reject(writeErr);
                        } else {
                            resolve({ path: tmpPath, cleanup });
                        }
                    });
                }
            },
        );
    });
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
