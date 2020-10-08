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

import { CommandResult } from "./cmd";

/**
 * CommandError is an error resulting from invocation of a Pulumi Command.
 * This is an opaque error that provides utility functions to detect specific error cases
 * such as concurrent stack updates (`isConcurrentUpdateError`). If you'd like to detect an additional
 * error case that isn't currently covered, please file an issue: https://github.com/pulumi/pulumi/issues/new
 * @alpha
 */
export class CommandError extends Error {
    /** @internal */
    constructor(private commandResult: CommandResult) {
        super(commandResult.toString());
        this.name = "CommandError";
    }
    /**
     * Returns true if the error was a result of a conflicting update locking the stack.
     *
     * @returns a boolean indicating if this failure was due to a conflicting concurrent update on the stack.
     */
    isConcurrentUpdateError() {
        return this.commandResult.stderr.indexOf("[409] Conflict: Another update is currently in progress.") >= 0;
    }
    /**
     * Returns true if the error was the result of selecting a stack that does not exist.
     *
     * @returns a boolean indicating if this failure was the result of selecting a stack that does not exist.
     */
    isSelectStack404Error() {
        const exp = new RegExp("no stack named.*found");
        return exp.test(this.commandResult.stderr);
    }
    /**
     * Returns true if the error was a result of creating a stack that already exists.
     *
     * @returns a boolean indicating if the error was a result of creating a stack that already exists.
     */
    isCreateStack409Error() {
        const exp = new RegExp("stack.*already exists");
        return exp.test(this.commandResult.stderr);
    }
}
