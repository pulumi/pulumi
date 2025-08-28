// Copyright 2016-2018, Pulumi Corporation.
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

export {
    // serializeFunctionAsync,
    // serializeFunction,
    SerializedFunction,
    SerializeFunctionArgs,
} from "./closure/serializeClosure";

export const serializeFunctionAsync = async (args: any) => {
    // FIXME: Bun uses the WebKit Inspector Protocol, not the v8 one.
    // lazy import, since this will end up pulling in the v8 module.
    const serializeClosure = await import("./closure/serializeClosure");
    return serializeClosure.serializeFunctionAsync(args);
}

export const serializeFunction = async (args: any) => {
    // FIXME: Bun uses the WebKit Inspector Protocol, not the v8 one.
    // lazy import, since this will end up pulling in the v8 module.
    const serializeClosure = await import("./closure/serializeClosure");
    return serializeClosure.serializeFunction(args);
}

export { CodePathOptions, computeCodePaths } from "./closure/codePaths";
export { Mocks, setMocks, MockResourceArgs, MockResourceResult, MockCallArgs, MockCallResult } from "./mocks";

export * from "./config";
export * from "./invoke";
export * from "./resource";
export * from "./rpc";
export * from "./settings";
export * from "./stack";
