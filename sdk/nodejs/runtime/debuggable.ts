// Copyright 2016, Pulumi Corporation.
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

import * as log from "../log";
import type { Resource } from "../resource";
import * as state from "./state";

/**
 * debugPromiseLeaks can be set to enable promises leaks debugging.
 *
 * @internal
 */
export const debugPromiseLeaks: boolean = !!process.env.PULUMI_DEBUG_PROMISE_LEAKS;

/**
 * leakDetectorScheduled is true when the promise leak detector is scheduled for
 * process exit.
 */
let leakDetectorScheduled: boolean = false;

function pendingRegistrationDescription(reg: state.PendingResourceRegistration): string {
    return reg.name && reg.type ? `"${reg.name}" [${reg.type}]` : reg.label;
}

function isDeferredOutput(v: unknown): boolean {
    return typeof v === "object" && v !== null && (v as any).__pulumiDeferredOutput === true;
}

function detectedCycleMessage(pending: Map<Resource, state.PendingResourceRegistration>): string | undefined {
    for (const reg of pending.values()) {
        for (const ownerReg of pendingOwners(reg.awaitingOutput, pending)) {
            const kind = waitCycleKind(ownerReg, reg, pending);
            if (kind !== undefined) {
                return cycleDescription(reg, ownerReg, kind);
            }
        }
    }
    return undefined;
}

function pendingOwners(
    output: unknown,
    pending: Map<Resource, state.PendingResourceRegistration>,
): Set<state.PendingResourceRegistration> {
    const owners = new Set<state.PendingResourceRegistration>();
    if (typeof output !== "object" || output === null) {
        return owners;
    }
    const addOwnersOf = (o: object) => {
        const resources = (o as any).resources;
        if (typeof resources !== "function") {
            return;
        }
        for (const res of resources.call(o) as Set<Resource>) {
            const ownerReg = pending.get(res);
            if (ownerReg !== undefined) {
                owners.add(ownerReg);
            }
        }
    };
    addOwnersOf(output);
    const source = state.getDeferredOutputSources().get(output);
    if (source !== undefined) {
        addOwnersOf(source);
    }
    return owners;
}

function waitCycleKind(
    start: state.PendingResourceRegistration,
    target: state.PendingResourceRegistration,
    pending: Map<Resource, state.PendingResourceRegistration>,
): "ancestry" | "inputs" | undefined {
    for (let cur: state.PendingResourceRegistration | undefined = start; cur !== undefined; ) {
        if (cur === target) {
            return "ancestry";
        }
        cur = cur.parent === undefined ? undefined : pending.get(cur.parent);
    }

    // Not reachable through parents alone: search the full wait-for graph, where a pending
    // registration waits on its parent's registration and on the owners of the output its
    // input serialization is currently awaiting.
    const visited = new Set<state.PendingResourceRegistration>([start]);
    const queue: state.PendingResourceRegistration[] = [start];
    while (queue.length > 0) {
        const cur = queue.shift()!;
        if (cur === target) {
            return "inputs";
        }
        const successors: state.PendingResourceRegistration[] = [];
        if (cur.parent !== undefined) {
            const parentReg = pending.get(cur.parent);
            if (parentReg !== undefined) {
                successors.push(parentReg);
            }
        }
        for (const ownerReg of pendingOwners(cur.awaitingOutput, pending)) {
            successors.push(ownerReg);
        }
        for (const next of successors) {
            if (!visited.has(next)) {
                visited.add(next);
                queue.push(next);
            }
        }
    }
    return undefined;
}

const registerOutputsRemedy = "Register the value with `registerResourceOutputs` instead of passing it as an input.";

function cycleDescription(
    reg: state.PendingResourceRegistration,
    ownerReg: state.PendingResourceRegistration,
    kind: "ancestry" | "inputs",
): string {
    const prop = reg.inputProperty !== undefined ? ` "${reg.inputProperty}"` : "";
    const component = pendingRegistrationDescription(reg);
    const other = pendingRegistrationDescription(ownerReg);
    let resolvedBy: string;
    if (ownerReg === reg) {
        resolvedBy =
            `from one of ${component}'s own outputs. The resource cannot be registered until this ` +
            "input resolves, and the output cannot resolve until the resource has been registered";
    } else if (kind === "inputs") {
        resolvedBy =
            `by an output of ${other}, and ${other}'s own registration is in turn waiting on ` +
            `${component} through its inputs or parent`;
    } else {
        resolvedBy =
            `by ${other}, a descendant of ${component}. ${component} cannot be registered until this ` +
            `input resolves, and ${other} cannot be registered until its ancestor ${component} has been registered`;
    }
    return (
        `input${prop} of resource ${component} is a deferred output that is resolved ${resolvedBy}, ` +
        `so the deployment would deadlock. ${registerOutputsRemedy}`
    );
}

function pendingRegistrationPhaseDescription(
    reg: state.PendingResourceRegistration,
    pending: Map<Resource, state.PendingResourceRegistration>,
): string {
    switch (reg.phase) {
        case "dependencies":
            return "waiting for its dependencies to resolve";
        case "inputs": {
            let suffix = "";
            if (isDeferredOutput(reg.awaitingOutput)) {
                suffix = state.getDeferredOutputSources().has(reg.awaitingOutput as object)
                    ? ", a deferred output"
                    : ", a deferred output that was never resolved";
            }
            return reg.inputProperty === undefined
                ? `waiting for its input properties to resolve${suffix}`
                : `waiting for the value of its input property "${reg.inputProperty}"${suffix}`;
        }
        case "parent": {
            const parentReg = reg.parent === undefined ? undefined : pending.get(reg.parent);
            const parent =
                parentReg !== undefined
                    ? pendingRegistrationDescription(parentReg)
                    : reg.parent?.__name !== undefined
                      ? `"${reg.parent.__name}"`
                      : "its parent";
            return `waiting for its parent ${parent} to finish registering`;
        }
        case "provider":
            return "waiting for its provider to finish registering";
        case "dependency-urns":
        default:
            return "waiting for the URNs of its dependencies to resolve";
    }
}

function fallbackCycleMessage(pending: Map<Resource, state.PendingResourceRegistration>): string {
    for (const reg of pending.values()) {
        if (isDeferredOutput(reg.awaitingOutput)) {
            return (
                "A deferred output that is never resolved, or that is resolved by a resource that directly or\n" +
                "indirectly waits on the registration awaiting it, can never complete. Make sure every deferred\n" +
                `output is resolved. ${registerOutputsRemedy}`
            );
        }
    }
    return (
        "This can happen when a resource's inputs, dependencies, or parent depend on a promise or\n" +
        "output that never resolves, or when there is a cyclic dependency between resources."
    );
}

function pendingRegistrationsMessage(pending: Map<Resource, state.PendingResourceRegistration>): string {
    const lines = [...pending.values()].map(
        (reg) => `  * ${pendingRegistrationDescription(reg)} was ${pendingRegistrationPhaseDescription(reg, pending)}`,
    );
    let message =
        "The Pulumi runtime detected that the program exited before the following resource\n" +
        "registrations could complete:\n" +
        lines.join("\n") +
        "\n\n";

    const detectedCycle = detectedCycleMessage(pending);
    if (detectedCycle !== undefined) {
        message += detectedCycle;
    } else {
        message += fallbackCycleMessage(pending);
    }

    return (
        message +
        "\n\n" +
        "Re-run your program with the `PULUMI_DEBUG_PROMISE_LEAKS` environment variable set for\n" +
        "additional debug information about the leaked promises."
    );
}

/**
 * @internal
 */
export function leakedPromises(): [Set<Promise<any>>, string] {
    const localStore = state.getStore();
    const leaked = localStore.leakCandidates;
    const pendingRegistrations = localStore.pendingResourceRegistrations;
    const promisePlural = leaked.size === 1 ? "promise was" : "promises were";
    let message =
        leaked.size === 0
            ? ""
            : `The Pulumi runtime detected that ${leaked.size} ${promisePlural} still active\n` +
              "at the time that the process exited. There are a few ways that this can occur:\n" +
              "  * Not using `await` or `.then` on a Promise returned from a Pulumi API\n" +
              "  * Introducing a cyclic dependency between two Pulumi Resources\n" +
              "  * A bug in the Pulumi Runtime\n" +
              "\n" +
              "Leaving promises active is probably not what you want. If you are unsure about\n" +
              "why you are seeing this message, re-run your program " +
              "with the `PULUMI_DEBUG_PROMISE_LEAKS`\n" +
              "environment variable. The Pulumi runtime will then print out additional\n" +
              "debug information about the leaked promises.";
    if (leaked.size > 0 && pendingRegistrations !== undefined && pendingRegistrations.size > 0) {
        message = pendingRegistrationsMessage(pendingRegistrations);
    }

    if (debugPromiseLeaks) {
        for (const leak of leaked) {
            console.error("Promise leak detected:");
            console.error(promiseDebugString(leak));
        }
    }

    localStore.leakCandidates = new Set();
    return [leaked, message];
}

/**
 * @internal
 */
export function promiseDebugString(p: Promise<any>): string {
    return `CONTEXT(${(<any>p)._debugId}): ${(<any>p)._debugCtx}\n` + `STACK_TRACE:\n` + `${(<any>p)._debugStackTrace}`;
}

let promiseId = 0;

/**
 * Optionally wraps a promise with some goo to make it easier to debug common
 * problems.
 *
 * @internal
 */
export function debuggablePromise<T>(p: Promise<T>, ctx: any): Promise<T> {
    const localStore = state.getStore();
    // Whack some stack onto the promise.  Leave them non-enumerable to avoid awkward rendering.
    Object.defineProperty(p, "_debugId", { writable: true, value: promiseId });
    Object.defineProperty(p, "_debugCtx", { writable: true, value: ctx });
    Object.defineProperty(p, "_debugStackTrace", { writable: true, value: new Error().stack });

    promiseId++;

    if (!leakDetectorScheduled) {
        process.on("exit", (code: number) => {
            // Only print leaks if we're exiting normally.  Otherwise, it could be a crash, which of
            // course yields things that look like "leaks".
            //
            // process.exitCode is undefined unless set, in which case it's the exit code that was
            // passed to process.exit.
            //
            // If `localStore.terminated` is true, the monitor was terminated while we were waiting
            // for an operation to complete. In this case we have in flight promises (the ones tied
            // to the operation), but we don't to report the leaks message. Pulumi will already have
            // notified the user of the error reason that lead to the monitor shutdown.
            if (
                (process.exitCode === undefined || process.exitCode === 0) &&
                !log.hasErrors() &&
                !localStore.terminated
            ) {
                const [leaks, message] = leakedPromises();
                if (leaks.size === 0) {
                    // No leaks - proceed with the exit.
                    return;
                }

                console.error(message);

                // Fail the deployment if we leaked any promises.
                process.exitCode = 1;
            }
        });
        leakDetectorScheduled = true;
    }

    // Add this promise to the leak candidates list, and schedule it for removal if it resolves.
    localStore.leakCandidates.add(p);
    return p
        .then((val: any) => {
            localStore.leakCandidates.delete(p);
            return val;
        })
        .catch((err: any) => {
            localStore.leakCandidates.delete(p);
            err.promise = p;
            throw err;
        });
}

/**
 * Produces a string from an error, conditionally including additional
 * diagnostics.
 *
 * @internal
 */
export function errorString(err: Error): string {
    if (err.stack) {
        return err.stack;
    }
    return err.toString();
}
