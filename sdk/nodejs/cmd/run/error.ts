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

import * as util from "util";

/**
 * @internal
 * defaultErrorMessage returns a string representation of the error. This includes
 * the full stack (which includes the message), or fallback to just the message
 * if we can't get the stack. If both the stack and message are empty, then just
 * stringify/inspect the err object itself. This is necessary as users can throw
 * arbitrary things in JS (including non-Errors).
 */
export function defaultErrorMessage(err: any): string {
    if (err?.stack) {
        // colorize stack trace if exists
        return util.inspect(err, { colors: true });
    }
    if (err?.message) {
        return err.message;
    }
    try {
        return "" + err;
    } catch {
        // If we can't stringify the error, use util.inspect to get a string representation.
        try {
            return util.inspect(err);
        } catch (error) {
            return `an error occurred while inspecting an error: ${error.message}`;
        }
    }
}
