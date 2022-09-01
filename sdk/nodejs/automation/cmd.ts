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
import { getStore } from "../runtime";

import { createCommandError } from "./errors";

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

const unknownErrCode = -2;

/** @internal */
export async function runPulumiCmd(
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

    const store = getStore();

    const config = !store?.config ? {} : store.config;

    const env = { ...config, ...additionalEnv };

    try {
        const proc = execa("pulumi", args, { env, cwd });

        if (onOutput && proc.stdout) {
            proc.stdout!.on("data", (data: any) => {
                if (data && data.toString) {
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
