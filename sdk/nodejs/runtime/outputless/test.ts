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

import { AsyncLocalStorage, AsyncResource, triggerAsyncId } from "node:async_hooks";
import { forkOutputlessState, getOutputlessDependencies, registerOutputlessDependencies } from ".";

async function main() {
    runTimer("main - before");

    new Promise(() => {
        runTimer("promise - before");
    });

    await new Promise<void>((resolve) => setTimeout(resolve, 1000));

    await registerOutputAsync();

    runTimer("main - after");

    new Promise(() => {
        runTimer("promise - after");
    });
}

function runTimer(prefix: string) {
    let timer: NodeJS.Timeout;

    let intervals = 0;
    // eslint-disable-next-line prefer-const
    timer = setInterval(() => {
        const currentDependencies = [...getOutputlessDependencies()];

        console.log(`${prefix} - current dependencies:`, currentDependencies.length);
        intervals += 1;
        if (intervals >= 10) {
            clearInterval(timer);
        }
    }, 200);
}

async function registerOutputAsync() {
    const dependencies = await new Promise<string[]>((resolve) => {
        setTimeout(() => resolve([""]), 500);
    });
    forkOutputlessState();
    registerOutputlessDependencies(dependencies);
}

async function registerOutput() {
    forkOutputlessState();
    registerOutputlessDependencies([{} as any]);
}

const ctx = new AsyncLocalStorage<string>();

function getParent() {
    return ctx.getStore();
}

class MyParentComponent {
    constructor(public type, public __name) {
        console.log("parent component?", getParent());
    }
}

class AsyncResourceCtx extends AsyncResource {
    constructor(public name: "test") {
        super("AsyncResourceCtx");
    }
}

async function componentMain() {
    new MyParentComponent("my:module:ParentComponent", "parent 1");

    console.log("component main 1", getParent());


    class foo extends AsyncResource {
        constructor(name: string) {
            super(name);
        }
    }

    triggerAsyncId()

    // @ts-ignore
    new Foo("my:module:ParentComponent", "parent 2");

    console.log("component main 2", getParent());
}

componentMain();

// main();
