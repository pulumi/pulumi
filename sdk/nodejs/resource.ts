// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as runtime from "./runtime";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

/**
 * Resource represents a class whose CRUD operations are implemented by a provider plugin.
 */
export abstract class Resource {
    /**
     * urn is the stable logical URN used to distinctly address a resource, both before and after deployments.
     */
    public readonly urn: Promise<URN>;
    /**
     * children tracks those resources that are created as a result of this one.  It may be appended to using
     * the adopt API, but the list is frozen as soon as the resource's final state has been computed.
     */
    public readonly children: Resource[];

    /**
     * Creates a new initialized resource object.
     */
    constructor() {
        this.children = [];
        runtime.initResource(this);
    }

    /**
     * finished returns true if registration has been completed for this resource.
     */
    private finished(): boolean {
        return runtime.isRegistered(this);
    }

    /**
     * Marks another resource as a child of this one.  This automatically tags resources that
     * are related to one another, for purposes of presentation, filtering, and so on.
     */
    protected adopt(child: Resource): void {
        if (this.finished()) {
            throw new Error("May not adopt new children after a resource's registration");
        }
        this.children.push(child);
    }

    /**
     * Creates and registers a new resource object.  t is the fully qualified type token and name is the "name" part
     * to use in creating a stable and globally unique URN for the object.  dependsOn is an optional list of other
     * resources that this resource depends on, controlling the order in which we perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param external True to indicate that this requires is external to Pulumi.
     * @param props The arguments to use to populate the new resource.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    protected register(t: string, name: string, external: boolean, props: ComputedValues, dependsOn?: Resource[]) {
        if (!t) {
            throw new Error("Missing resource type argument");
        }
        if (!name) {
            throw new Error("Missing resource name argument (for URN creation)");
        }

        // Now kick off the resource registration.  If we are actually performing a deployment, this resource's
        // properties will be resolved asynchronously after the operation completes, so that dependent computations
        // resolve normally.  If we are just planning, on the other hand, values will never resolve.
        runtime.registerResource(this, t, name, external, props, this.children, dependsOn);
    }
}

/**
 * ExternalResource is a resource whose create, read, update, and delete (CRUD) operations are managed by performing
 * external operations on some physical entity.  The engine understands how to diff and perform partial updates of
 * them, and these CRUD operations are implemented in a dynamically loaded plugin for the defining package.
 */
export abstract class ExternalResource extends Resource {
    /**
     * id is the provider-assigned unique ID for this managed resource.  It is set during deployments and may be
     * missing (undefined) during planning phases.
     */
    public readonly id: Computed<ID>;

    /**
     * Creates and registers a new managed resource.  t is the fully qualified type token and name is the "name" part
     * to use in creating a stable and globally unique URN for the object.  dependsOn is an optional list of other
     * resources that this resource depends on, controlling the order in which we perform resource operations.
     * Creating an instance does not necessarily perform a create on the physical entity which it represents, and
     * instead, this is dependent upon the diffing of the new goal state compared to the current known resource state.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    constructor(t: string, name: string, props: ComputedValues, dependsOn?: Resource[]) {
        super();
        this.register(t, name, true, props, dependsOn);
    }
}

/**
 * Maybe is a union of either a T or undefined.
 */
export type Maybe<T> = T | undefined;

/**
 * Computed is a property output for a resource.  It is just a promise that also permits undefined values.  The
 * undefined values are used during planning, when the actual final value of a resource may not yet be known.
 */
export type Computed<T> = Promise<Maybe<T>>;

/**
 * ComputedValue is a property input for a resource.  It may be a promptly available T or a promise for one.
 */
export type ComputedValue<T> = Maybe<T> | Computed<T> | Promise<T>;

/**
 * ComputedValues is a map of property name to optional property input, one for each resource property value.
 */
export type ComputedValues = {[key: string]: ComputedValue<any> | undefined};

