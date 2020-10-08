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

import * as childProcess from "child_process";

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
export function runPulumiCmd(
    args: string[],
    cwd: string,
    additionalEnv: { [key: string]: string },
    onOutput?: (data: string) => void,
): Promise<CommandResult> {
    // all commands should be run in non-interactive mode.
    // this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
    args.push("--non-interactive");
    const env = { ...process.env, ...additionalEnv };

    return new Promise<CommandResult>((resolve, reject) => {
        const proc = childProcess.spawn("pulumi", args, { env, cwd });

        // TODO: write to buffers and avoid concatenation
        let stdout = "";
        let stderr = "";
        proc.stdout.on("data", (data) => {
            if (data && data.toString) {
                data = data.toString();
            }
            if (onOutput) {
                onOutput(data);
            }
            stdout += data;
        });
        proc.stderr.on("data", (data) => {
            stderr += data;
        });
        proc.on("exit", (code, signal) => {
            const resCode = code !== null ? code : unknownErrCode;
            const result = new CommandResult(stdout, stderr, resCode);
            if (code !== 0) {
                return reject(createCommandError(result));
            }
            return resolve(result);
        });
        proc.on("error", (err) => {
            const result = new CommandResult(stdout, stderr, unknownErrCode, err);
            return reject(createCommandError(result));
        });
    });
}
