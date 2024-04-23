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

import { ComponentResource, ComponentResourceOptions, mergeOptions } from "../../resource";
import { enterForkedOutputlessState } from "../outputless";
import { registerResourceOutputs } from "../resource";
import { AsyncComponentContext } from "./context";
import { defaultNamingConvention } from "./defaultNamingConvention";

/**
 * Wrap a class as a Pulumi ComponentResource.
 */
export function Component<
    Class extends new (name: string, inputs: any, opts: ComponentResourceOptions, ...args: any[]) => InstanceType<Class> & object
>(type: string) {
    return (value: Class, _decoratorContext: any): Class => {
        // It's illegal to call super() in a nested function call, ... so we don't! We are going old
        // school here. Instead declaring a pre-ES6 class using prototypes.
        //
        // We declare a function and give it the prototype of the wrapped class. The decorator wraps
        // returns a subclass of the wrapped class, but with a constructor that uses async context
        // to track the complete registration of all child resources.
        const wrapperFunction = function (name: string, inputs: any, opts: ComponentResourceOptions, ...args: any[]) {
            opts = mergeOptions({ namingConvention: defaultNamingConvention}, opts);

            const parent = new ComponentResource(type, name, {}, opts);

            opts = mergeOptions({ parent }, opts);

            const context = new AsyncComponentContext(parent);
            // Initialize an async context and start tracking.
            return context.run(() => {
                enterForkedOutputlessState();
                let constructed = new value(name, inputs, opts, ...args);
                let outputEntries = Object.keys(constructed)
                    .filter((key) => {
                        if (key.startsWith("__")) {
                            return false;
                        }
                        if (typeof key === "function") {
                            return false;
                        }

                        return true;
                    })
                    .map((key) => [key, (<any>constructed)[key]]);

                // @ts-ignore - Node 18 supports this, but TS doesn't yet.
                const outputs = Object.fromEntries(outputEntries);

                context.asyncCompletion.then(() => {
                    registerResourceOutputs(parent, outputs);
                });

                return constructed;
            });
        };
        wrapperFunction.prototype = value.prototype;

        return wrapperFunction as any;
    };
}
