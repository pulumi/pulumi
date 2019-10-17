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

import { ProviderResource, Resource } from "./resource";

/*
 * InvokeOptions is a bag of options that control the behavior of a call to runtime.invoke.
 */
export interface InvokeOptions {
    /**
     * An optional parent to use for default options for this invoke (e.g. the default provider to use).
     */
    parent?: Resource;

    /**
     * An optional provider to use for this invocation. If no provider is supplied, the default provider for the
     * invoked function's package will be used.
     */
    provider?: ProviderResource;

    /**
     * An optional version, corresponding to the version of the provider plugin that should be used when performing this
     * invoke.
     */
    version?: string;

    /**
     * Invoke this function asynchronously.  If 'true' is passed in here, then an invoked function
     * will only supply the `Promise<>` side of its result.
     */
    async?: boolean;
}
