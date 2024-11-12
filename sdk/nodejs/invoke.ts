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

import { Inputs, Input } from "./output";
import { ProviderResource, Resource } from "./resource";

/**
 * {@link InvokeOptions} is a bag of options that control the behavior of a call
 * to `runtime.invoke`.
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
     * An optional version, corresponding to the version of the provider plugin
     * that should be used when performing this invoke.
     */
    version?: string;

    /**
     *  An option to specify the URL from which to download this resources
     * associated plugin. This version overrides the URL information inferred
     * from the current package and should rarely be used.
     */
    pluginDownloadURL?: string;

    /**
     * Invoke this data source function asynchronously.  Defaults to `true` if unspecified.
     *
     * When `true`, only the `Promise<>` side of the invoke result is present.  Explicitly pass in
     * `false` to get the non-Promise side of the result.  Invoking data source functions
     * synchronously is deprecated.  The ability to do this will be removed at a later point in
     * time.
     */
    async?: boolean;
}

/**
 * {@link InvokeOutputOptions} is a bag of options that control the behavior of a call
 * to `runtime.invokeOutput`.
 */
export interface InvokeOutputOptions extends InvokeOptions {
    /**
     * An optional set of additional explicit dependencies on other resources.
     */
    dependsOn?: Input<Input<Resource>[]> | Input<Resource>;
}

/**
 * {@link InvokeTransform} is the callback signature for the `transforms`
 * resource option for invokes.  A transform is passed the same set of inputs
 * provided to the {@link Invoke} constructor, and can optionally return back
 * alternate values for the `args` and/or `opts` prior to the invoke actually
 * being executed.  The effect will be as though those args and opts were passed
 * in place of the original call to the {@link Invoke}.  If the transform
 * returns nil, this indicates
 * that the Invoke
 */
export type InvokeTransform = (
    args: InvokeTransformArgs,
) => Promise<InvokeTransformResult | undefined> | InvokeTransformResult | undefined;

/**
 * {@link InvokeTransformArgs} is the argument bag passed to a invoke transform.
 */
export interface InvokeTransformArgs {
    /**
     * The token of the Invoke.
     */
    token: string;
    /**
     * The original args passed to the Invoke constructor.
     */
    args: Inputs;
    /**
     * The original invoke options passed to the Invoke constructor.
     */
    opts: InvokeOptions;
}

/**
 * {@link InvokeTransformResult} is the result that must be returned by an invoke
 * transform callback.  It includes new values to use for the `args` and `opts`
 * of the `Invoke` in place of the originally provided values.
 */
export interface InvokeTransformResult {
    /**
     * The new properties to use in place of the original `args`
     */
    args: Inputs;
    /**
     * The new resource options to use in place of the original `opts`
     */
    opts: InvokeOptions;
}
