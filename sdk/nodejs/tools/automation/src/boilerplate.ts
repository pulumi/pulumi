// Copyright 2026-2026, Pulumi Corporation.
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

import { CommandResult, PulumiCommand } from "../../../automation/cmd";

export interface PulumiOptionsBase {
  command: PulumiCommand,
  cwd?: string,
  additionalEnv: { [key: string]: string },
  onOutput?: (output: string) => void,
  onError?: (error: string) => void,
  signal?: AbortSignal,
}

// Execute the given command and return the process output.
async function __run(options: PulumiOptionsBase, args: string[]): Promise<CommandResult> {
  return options.command.run(
    args,
    options.cwd ?? process.cwd(),
    options.additionalEnv,
    options.onOutput,
    options.onError,
    options.signal
  );
}
