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

export type CommandResult = {
    stdout: string,
    stderr: string,
    code: number,
    err?: Error,
};

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
            if (onOutput) {
                onOutput(data);
            }
            stdout += data;
        });
        proc.stderr.on("data", (data) => {
            stderr += data;
        });
        proc.on("exit", (code, signal) => {
            const result: CommandResult = {
                stdout,
                stderr,
                code : code !== null ? code : -1,
            };
            if (code !== 0) {
                return reject(result);
            }
            return resolve(result);
        });
        proc.on("error", (err) => {
            const result: CommandResult = {
                stdout,
                stderr,
                code : -1,
                err,
            };
            return reject(result);
        });
    });
}
