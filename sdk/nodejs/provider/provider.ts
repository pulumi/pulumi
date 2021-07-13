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

import { Input, Inputs } from "../output";
import * as resource from "../resource";
import * as runtime from "../runtime";

/**
 * CheckResult represents the results of a call to `ResourceProvider.check`.
 */
export interface CheckResult {
    /**
     * The inputs to use, if any.
     */
    readonly inputs?: any;

    /**
     * Any validation failures that occurred.
     */
    readonly failures?: CheckFailure[];
}

/**
 * CheckFailure represents a single failure in the results of a call to `ResourceProvider.check`
 */
export interface CheckFailure {
    /**
     * The property that failed validation.
     */
    readonly property: string;

    /**
     * The reason that the property failed validation.
     */
    readonly reason: string;
}

/**
 * DiffResult represents the results of a call to `ResourceProvider.diff`.
 */
export interface DiffResult {
    /**
     * If true, this diff detected changes and suggests an update.
     */
    readonly changes?: boolean;

    /**
     * If this update requires a replacement, the set of properties triggering it.
     */
    readonly replaces?: string[];

    /**
     * An optional list of properties that will not ever change.
     */
    readonly stables?: string[];

    /**
     * If true, and a replacement occurs, the resource will first be deleted before being recreated.  This is to
     * avoid potential side-by-side issues with the default create before delete behavior.
     */
    readonly deleteBeforeReplace?: boolean;
}

/**
 * CreateResult represents the results of a call to `ResourceProvider.create`.
 */
export interface CreateResult {
    /**
     * The ID of the created resource.
     */
    readonly id: resource.ID;

    /**
     * Any properties that were computed during creation.
     */
    readonly outs?: any;
}

export interface ReadResult {
    /**
     * The ID of the resource ready back (or blank if missing).
     */
    readonly id?: resource.ID;
    /**
     * The current property state read from the live environment.
     */
    readonly props?: any;
}

/**
 * UpdateResult represents the results of a call to `ResourceProvider.update`.
 */
export interface UpdateResult {
    /**
     * Any properties that were computed during updating.
     */
    readonly outs?: any;
}

/**
 * ConstructResult represents the results of a call to `ResourceProvider.construct`.
 */
export interface ConstructResult {
    /**
     * The URN of the constructed resource.
     */
    readonly urn: Input<resource.URN>;

    /**
     * Any state that was computed during construction.
     */
    readonly state: Inputs;
}

/**
 * InvokeResult represents the results of a call to `ResourceProvider.invoke`.
 */
export interface InvokeResult {
    /**
     * The outputs returned by the invoked function, if any.
     */
    readonly outputs?: any;

    /**
     * Any validation failures that occurred.
     */
    readonly failures?: CheckFailure[];
}

/**
 * Provider represents an object that implements the resources and functions for a particular Pulumi package.
 */
export interface Provider {
    /**
     * The version of the provider. Must be valid semver.
     */
    version: string;

    /**
     * The JSON-encoded schema for this provider's package.
     */
    schema?: string;

    /**
     * Check validates that the given property bag is valid for a resource of the given type.
     *
     * @param olds The old input properties to use for validation.
     * @param news The new input properties to use for validation.
     */
    check?: (urn: resource.URN, olds: any, news: any) => Promise<CheckResult>;

    /**
     * Diff checks what impacts a hypothetical update will have on the resource's properties.
     *
     * @param id The ID of the resource to diff.
     * @param olds The old values of properties to diff.
     * @param news The new values of properties to diff.
     */
    diff?: (id: resource.ID, urn: resource.URN, olds: any, news: any) => Promise<DiffResult>;

    /**
     * Create allocates a new instance of the provided resource and returns its unique ID afterwards.
     * If this call fails, the resource must not have been created (i.e., it is "transactional").
     *
     * @param inputs The properties to set during creation.
     */
    create?: (urn: resource.URN, inputs: any) => Promise<CreateResult>;

    /**
     * Reads the current live state associated with a resource.  Enough state must be included in the inputs to uniquely
     * identify the resource; this is typically just the resource ID, but it may also include some properties.
     */
    read?: (id: resource.ID, urn: resource.URN, props?: any) => Promise<ReadResult>;

    /**
     * Update updates an existing resource with new values.
     *
     * @param id The ID of the resource to update.
     * @param olds The old values of properties to update.
     * @param news The new values of properties to update.
     */
    update?: (id: resource.ID, urn: resource.URN, olds: any, news: any) => Promise<UpdateResult>;

    /**
     * Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
     *
     * @param id The ID of the resource to delete.
     * @param props The current properties on the resource.
     */
    delete?: (id: resource.ID, urn: resource.URN, props: any) => Promise<void>;

    /**
     * Construct creates a new component resource.
     *
     * @param name The name of the resource to create.
     * @param type The type of the resource to create.
     * @param inputs The inputs to the resource.
     * @param options the options for the resource.
     */
    construct?: (name: string, type: string, inputs: Inputs, options: resource.ComponentResourceOptions)
        => Promise<ConstructResult>;

    /**
     * Call calls the indicated method.
     *
     * @param token The token of the method to call.
     * @param inputs The inputs to the method.
     */
    call?: (token: string, inputs: Inputs) => Promise<InvokeResult>;

    /**
     * Invoke calls the indicated function.
     *
     * @param token The token of the function to call.
     * @param inputs The inputs to the function.
     */
    invoke?: (token: string, inputs: any) => Promise<InvokeResult>;
}
