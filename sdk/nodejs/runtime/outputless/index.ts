// Copyright 2016-2024, Pulumi Corporation.
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
import { getLocalStore } from "../state";

/**
 * To avoid import cycles, we define a simple URN type and avoid importing Resource or any other
 * types.
 */
type urn = string;

/**
 * Tracks dependencies for outputless Pulumi. Tracking dependencies in an async local store allows
 * us to inject those dependencies into subsequent resource creations.
 *
 * @internal
 */
export class OutputlessState {
    #dependencies = new Set<urn>();

    addDependencies(resources: urn[]) {
        for (const res of resources) {
            this.#dependencies.add(res);
        }
    }

    getDependencies() {
        return this.#dependencies;
    }

    clone() {
        const newState = new OutputlessState();
        newState.#dependencies = new Set(this.#dependencies);
        return newState;
    }
}

const outputlessStore = new AsyncLocalStorage<OutputlessState>();

/**
 * Returns the current outputless state, i.e.: the set of ambient dependencies.
 *
 * @internal
 */
function getOutputlessState() {
    const state = outputlessStore.getStore() ?? getLocalStore()?.outputlessState;
    if (!state) {
        // console.log("creating new outputless state");
        return new OutputlessState();
    }
    // console.log("reusing outputless state");
    return state;
}

/**
 * Clones the current outputless state and enters it into the async local store.
 *
 * @internal
 */
export function enterForkedOutputlessState() {
    const state = getOutputlessState();
    outputlessStore.enterWith(state.clone());
}


/**
 * Weaker version of "Awaited" because we are on an old TypeScript version.
 *
 * This unwraps ony level of a PromiseLike type.
 */
type Awaited<T> = T extends null | undefined ? T : // special case for `null | undefined` when not in `--strictNullChecks` mode
T extends object & { then(onfulfilled: infer F, ...args: infer _): any; } ? // `await` only unwraps object types with a callable `then`. Non-object types are not unwrapped
    F extends ((value: infer V, ...args: infer _) => any) ? // if the argument to `then` is callable, extracts the first argument
        V : // ⬅️ Awaited<V> in TypeScript 4.5+
    never : // the argument to `then` was not callable
T; // non-object or non-thenable


/**
 * Runs a function within a new outputless context.
 *
 * When the result of this function is awaited, the ambient dependencies registered within the
 * function are registered in the parent context.
 */
export function forkPulumiContext<F extends (...args: any[]) => any>(fn: F, ...args: Parameters<F>): PromiseLike<Awaited<ReturnType<F>>> {
    const parentContext = getOutputlessState();
    const childContext = parentContext.clone();
    const result = outputlessStore.run(childContext, fn, ...args);
    return {
        then<TResult1 = Awaited<ReturnType<F>>, TResult2 = never>(
            onfulfilled?: ((value: Awaited<ReturnType<F>>) => TResult1 | PromiseLike<TResult1>) | null | undefined,
            onrejected?: ((reason: any) => TResult2 | PromiseLike<TResult2>) | null | undefined,
        ): PromiseLike<TResult1 | TResult2> {
            parentContext.addDependencies([...childContext.getDependencies()]);
            return result.then(onfulfilled, onrejected);
        }
    }
}

/**
 * Registers ambient dependencies to include in subsequent resource registrations.
 *
 * @internal
 */
export function registerOutputlessDependencies(resources: urn[]) {
    getOutputlessState().addDependencies(resources);
}

/**
 * Get the current set of dependencies.
 *
 * @internal
 */
export function getOutputlessDependencies(): Iterable<urn> {
    const dependencies = outputlessStore.getStore()?.getDependencies() ?? [];
    // console.log("getting outputless dependencies", dependencies);
    return dependencies;
}
