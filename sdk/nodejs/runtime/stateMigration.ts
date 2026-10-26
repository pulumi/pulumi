// Copyright 2026, Pulumi Corporation.
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

import { AsyncLocalStorage } from "async_hooks";

const activeStateMigration = new AsyncLocalStorage<string>();

/** @internal */
export function runStateMigration<T>(urn: string, callback: () => T): T {
    return activeStateMigration.run(urn, callback);
}

/** @internal */
export function ensureNotInStateMigration(operation: string): void {
    const urn = activeStateMigration.getStore();
    if (urn === undefined) {
        return;
    }

    throw new Error(
        `Pulumi runtime operation '${operation}' is not allowed inside the state migration callback for '${urn}'. ` +
            "State migration callbacks must only transform the supplied state.",
    );
}
