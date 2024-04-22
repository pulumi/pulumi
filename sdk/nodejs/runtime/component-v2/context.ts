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

import { AsyncHook, AsyncLocalStorage, createHook, executionAsyncId } from "async_hooks";
import type { ComponentResource } from "../..";
import { PromiseResolvers, promiseWithResolvers } from "./promiseWithResolvers";

const asyncComponentContext = new AsyncLocalStorage<AsyncComponentContext>();


let instanceId = 0;
/**
 * @internal
 */
export class AsyncComponentContext {
    #instanceId = instanceId++;
    #asyncIds: Set<number> = new Set();
    #resolved: boolean = false;
    #asyncCompletion: PromiseResolvers<void> = promiseWithResolvers();
    #componentResource: ComponentResource | undefined;

    get asyncCompletion() {
        return this.#asyncCompletion.promise;
    }

    get registeredParent() {
        return this.#componentResource;
    }

    constructor(parent: ComponentResource) {
        this.#componentResource = parent;
        asyncHook.enable();
    }

    run<R>(fn: (...args: any[]) => R, ...args: any[]): R {
        return asyncComponentContext.run(this, fn, ...args);
    }

    registerAsyncId(asyncId: number, type: any, triggerAsyncId: number) {
        // n.b.: logging in this method can cause a stack overflow, as it is called for every async
        // operation, including writing to stdout.
        this.#asyncIds.add(asyncId);
    }

    unregisterAsyncId(asyncId: number) {
        // console.log(`${this.#instanceId} - Leaving async context ${asyncId}`);
        this.#asyncIds.delete(asyncId);
        if (this.#asyncIds.size !== 0) {
            return;
        }
        if (this.#resolved) {
            return;
        }

        this.#resolved = true;
        this.#asyncCompletion.resolve();
    }
}

/**
 * @internal
 */

export function getAsyncComponentParent(): ComponentResource | undefined {
    return asyncComponentContext.getStore()?.registeredParent;
}
/**
 * @internal
 */

export function getAsyncComponentCompletion(): Promise<void> | undefined {
    return asyncComponentContext.getStore()?.asyncCompletion;
}

/**
 * Async hook that tracks all spawned async contexts. In a wrapped async component context, if any
 * task is spawned by Node.js - promises, emitters, etc., - it will be tracked here.
 *
 * When all async tasks are completed, the children of the component are resolved.
 *
 * @internal
 */
export const asyncHook: AsyncHook = createHook({
    init(asyncId, type, triggerAsyncId, resource) {
        const context = asyncComponentContext.getStore();
        if (context) {
            context.registerAsyncId(asyncId, type, triggerAsyncId);
        }

    },
    destroy(asyncId) {
        const context = asyncComponentContext.getStore();
        if (context) {
            context.unregisterAsyncId(asyncId);
        }
    },
    promiseResolve(asyncId) {
        const context = asyncComponentContext.getStore();
        if (context) {
            context.unregisterAsyncId(asyncId);
        }
    },
});
