// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as resource from "../resource";
import * as runtime from "../runtime";

/**
 * CheckResult represents the results of a call to `ResourceProvider.check`.
 */
export class CheckResult {
    /**
     * The inputs to use, if any.
     */
    public readonly inputs: any | undefined;

    /**
     * Any validation failures that occurred.
     */
    public readonly failures: CheckFailure[];

    /**
     * Constructs a new check result.
     *
     * @param inputs The inputs to use, if any.
     * @param failures Any validation failures that occurred.
     */
    constructor(inputs: any | undefined, failures: CheckFailure[]) {
        this.inputs = inputs;
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
     * Constructs a new update result.
     *
     * @param outs Any properties that were computed during updating.
     */
    constructor(outs: any | undefined) {
        this.outs = outs;
    }
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
    check: (olds: any, news: any) => Promise<CheckResult>;

    /**
     * Diff checks what impacts a hypothetical update will have on the resource's properties.
     *
     * @param id The ID of the resource to diff.
     * @param olds The old values of properties to diff.
     * @param news The new values of properties to diff.
     */
    diff: (id: resource.ID, olds: any, news: any) => Promise<DiffResult>;

    /**
     * Create allocates a new instance of the provided resource and returns its unique ID afterwards.
     * If this call fails, the resource must not have been created (i.e., it is "transacational").
     *
     * @param inputs The properties to set during creation.
     */
    create: (inputs: any) => Promise<CreateResult>;

    /**
     * Update updates an existing resource with new values.
     *
     * @param id The ID of the resource to update.
     * @param olds The old values of properties to update.
     * @param news The new values of properties to update.
     */
    update: (id: resource.ID, olds: any, news: any) => Promise<UpdateResult>;

    /**
     * Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
     *
     * @param id The ID of the resource to delete.
     * @param props The current properties on the resource.
     */
    delete: (id: resource.ID, props: any) => Promise<void>;
}

async function serializeProvider(provider: ResourceProvider): Promise<string> {
    return runtime.serializeJavaScriptText(await runtime.serializeClosure(() => provider));
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
     * @param parent An optional parent resource to which this resource belongs.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    public constructor(provider: ResourceProvider,
                       name: string,
                       props: resource.ComputedValues,
                       parent?: resource.Resource,
                       dependsOn?: resource.Resource[]) {
        const providerKey: string = "__provider";

        if (props[providerKey]) {
            throw new Error("A dynamic resource must not define the __provider key");
        }
        props[providerKey] = serializeProvider(provider);

        super("pulumi-nodejs:dynamic:Resource", name, props, parent, dependsOn);
    }
}
