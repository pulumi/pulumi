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

import { Inputs } from "../output";
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
     * void potential side-by-side issues with the default create before delete behavior.
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
 * ResourceProvider represents an object that provides CRUD operations for a particular type of resource.
 */
export interface ResourceProvider {
    /**
     * Check validates that the given property bag is valid for a resource of the given type.
     *
     * @param olds The old input properties to use for validation.
     * @param news The new input properties to use for validation.
     */
    check?: (olds: any, news: any) => Promise<CheckResult>;

    /**
     * Diff checks what impacts a hypothetical update will have on the resource's properties.
     *
     * @param id The ID of the resource to diff.
     * @param olds The old values of properties to diff.
     * @param news The new values of properties to diff.
     */
    diff?: (id: resource.ID, olds: any, news: any) => Promise<DiffResult>;

    /**
     * Create allocates a new instance of the provided resource and returns its unique ID afterwards.
     * If this call fails, the resource must not have been created (i.e., it is "transacational").
     *
     * @param inputs The properties to set during creation.
     */
    create: (inputs: any) => Promise<CreateResult>;

    /**
     * Reads the current live state associated with a resource.  Enough state must be included in the inputs to uniquely
     * identify the resource; this is typically just the resource ID, but it may also include some properties.
     */
    read?: (id: resource.ID, props?: any) => Promise<ReadResult>;

    /**
     * Update updates an existing resource with new values.
     *
     * @param id The ID of the resource to update.
     * @param olds The old values of properties to update.
     * @param news The new values of properties to update.
     */
    update?: (id: resource.ID, olds: any, news: any) => Promise<UpdateResult>;

    /**
     * Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
     *
     * @param id The ID of the resource to delete.
     * @param props The current properties on the resource.
     */
    delete?: (id: resource.ID, props: any) => Promise<void>;
}

function serializeProvider(provider: ResourceProvider): Promise<string> {
    return runtime.serializeFunction(() => provider).then(sf => sf.text);
}

/**
 * Resource represents a Pulumi Resource that incorporates an inline implementation of the Resource's CRUD operations.
 */
export abstract class Resource extends resource.CustomResource {
    /**
     * Creates a new dynamic resource.
     *
     * @param provider The implementation of the resource's CRUD operations.
     * @param name The name of the resource.
     * @param props The arguments to use to populate the new resource. Must not define the reserved
     *              property "__provider".
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(provider: ResourceProvider, name: string, props: Inputs,
                opts?: resource.CustomResourceOptions) {
        const providerKey: string = "__provider";
        if (props[providerKey]) {
            throw new Error("A dynamic resource must not define the __provider key");
        }
        props[providerKey] = serializeProvider(provider);

        super("pulumi-nodejs:dynamic:Resource", name, props, opts);
    }
}
