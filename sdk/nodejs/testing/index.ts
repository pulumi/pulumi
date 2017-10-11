// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as resource from "../resource";

/**
 * ProvidersConfigKey is the configuration key used to provide the testing provider with the path to a JavaScript module
 * that exports a map from (unqualified) type names to `ResourceProvider`s. This map is then used to decide which
 * `ResourceProvider` should be used to implement the CRUD operations for a particular resource type.
 */
export const ProvidersConfigKey = "testing:providers:module"

/**
 * CheckResult represents the results of a call to `ResourceProvider.check`.
 */
export class CheckResult {
    /**
     * The defaults to use, if any.
     */
    public readonly defaults: any | undefined;

    /**
     * Any validation failures that occurred.
     */
    public readonly failures: CheckFailure[];

    /**
     * Constructs a new check result.
     *
     * @param defaults The defaults to use, if any.
     * @param failures Any validation failures that occurred.
     */
    constructor(defaults: any | undefined, failures: CheckFailure[]) {
        this.defaults = defaults;
        this.failures = failures;
    }
}

/**
 * CheckFailure represents a single failure in the results of a call to `ResourceProvider.check`
 */
export class CheckFailure {
    /**
     * The property that failed validation.
     */
    public readonly property: string;

    /**
     * The reason that the property failed validation.
     */
    public readonly reason: string;

    /**
     * Constructs a new check failure.
     *
     * @param property The property that failed validation.
     * @param reason The reason that the property failed validation.
     */
    constructor(property: string, reason: string) {
        this.property = property;
        this.reason = reason;
    }
}

/**
 * DiffResult represents the results of a call to `ResourceProvider.diff`.
 */
export class DiffResult {
    /**
     * If this update requires a replacement, the set of properties triggering it.
     */
    public readonly replaces: string[];

    /**
     * An optional list of properties that will not ever change.
     */
    public readonly stables: string[];

    /**
     * Constructs a new diff result.
     *
     * @param replaces If this update requires a replacement, the set of properties triggering it.
     * @param stables An optional list of properties that will not ever change.
     */
    constructor(replaces: string[], stables: string[]) {
        this.replaces = replaces;
        this.stables = stables;
    }
}

/**
 * CreateResult represents the results of a call to `ResourceProvider.create`.
 */
export class CreateResult {
    /**
     * The ID of the created resource.
     */
    public readonly id: resource.ID;

    /**
     * Any properties that were computed during creation.
     */
    public readonly outs: any | undefined;

    /**
     * Constructs a new create result.
     *
     * @param id The ID of the created resource.
     * @param outs Any properties that were computed during creation.
     */
    constructor(id: resource.ID, outs: any | undefined) {
        this.id = id;
        this.outs = outs;
    }
}

/**
 * UpdateResult represents the results of a call to `ResourceProvider.update`.
 */
export class UpdateResult {
    /**
     * Any properties that were computed during updating.
     */
    public readonly outs: any | undefined;

    /**
     * Constructs a new udpate result.
     *
     * @param outs Any properties that were computed during updating.
     */
    constructor(outs: any | undefined) {
        this.outs = outs;
    }
}

/**
 * ResourceProvider represents an object that provides CRUD operations for a particular
 */
export interface ResourceProvider {
    /**
     * Check validates that the given property bag is valid for a resource of the given type.
     *
     * @param inputs The full properties to use for validation.
     */
    check(inputs: any): Promise<CheckResult>;

    /**
     * Diff checks what impacts a hypothetical update will have on the resource's properties.
     *
     * @param id The ID of the resource to diff.
     * @param olds The old values of properties to diff.
     * @param news The new values of properties to diff.
     */
    diff(id: resource.ID, olds: any, news: any): Promise<DiffResult>;

    /**
     * Create allocates a new instance of the provided resource and returns its unique ID afterwards.
     * If this call fails, the resource must not have been created (i.e., it is "transacational").
     *
     * @param inputs The properties to set during creation.
     */
    create(inputs: any): Promise<CreateResult>;

    /**
     * Update updates an existing resource with new values.
     *
     * @param id The ID of the resource to update.
     * @param olds The old values of properties to update.
     * @param news The new values of properties to update.
     */
    update(id: resource.ID, olds: any, news: any): Promise<UpdateResult>;

    /**
     * Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
     *
     * @param id The ID of the resource to delete.
     * @param props The current properties on the resource.
     */
    delete(id: resource.ID, props: any): Promise<void>;
}
