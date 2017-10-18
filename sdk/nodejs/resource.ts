// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as runtime from "./runtime";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

/**
 * Resource represents a class whose CRUD operations are implemented by a provider plugin.
 */
export abstract class Resource {
    /**
     * parentScope tracks the currently active parent to automatically parent children to.
     */
    private static parentScope: (Resource | undefined)[] = [];

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
     * runInParentScope executes a callback, body, and any resources allocated within become parent's children.
     */
    public static runInParentScope<T>(parent: Resource, body: () => T): T {
        Resource.parentScope.push(parent);
        try {
            return body();
        }
        finally {
            Resource.parentScope.pop();
        }
    }

    /**
     * runInParentlessScope executes a callback, body, in a scope where no parent is active.  This can be useful
     * if there's an active parent but you want to run some code that allocates "anonymous" resources.
     */
    public static runInParentlessScope<T>(body: () => T): T {
        Resource.parentScope.push(undefined);
        try {
            return body();
        }
        finally {
            Resource.parentScope.pop();
        }
    }

    /**
     * Creates a new initialized resource object.
     */
    constructor() {
        this.children = [];
        runtime.initResource(this);

        // If there is a parent scope, automatically add this to it as a child.
        if (Resource.parentScope.length) {
            const parent: Resource | undefined = Resource.parentScope[Resource.parentScope.length-1];
            if (parent) {
                parent.adopt(this);
            }
        }
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
     * @param custom True to indicate that this is a custom resource, managed by a plugin.
     * @param props The arguments to use to populate the new resource.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    protected register(t: string, name: string, custom: boolean, props: ComputedValues, dependsOn?: Resource[]) {
        if (!t) {
            throw new Error("Missing resource type argument");
        }
        if (!name) {
            throw new Error("Missing resource name argument (for URN creation)");
        }

        // Now kick off the resource registration.  If we are actually performing a deployment, this resource's
        // properties will be resolved asynchronously after the operation completes, so that dependent computations
        // resolve normally.  If we are just planning, on the other hand, values will never resolve.
        runtime.registerResource(this, t, name, custom, props, this.children, dependsOn);
    }
}

/**
 * CustomResource is a resource whose create, read, update, and delete (CRUD) operations are managed by performing
 * external operations on some physical entity.  The engine understands how to diff and perform partial updates of
 * them, and these CRUD operations are implemented in a dynamically loaded plugin for the defining package.
 */
export abstract class CustomResource extends Resource {
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
 * ComponentResource is a resource that aggregates one or more other child resources into a higher level abstraction.
 * The component resource itself is a resource, but does not require custom CRUD operations for provisioning.
 */
export abstract class ComponentResource extends Resource {
    /**
     * Creates and registers a new component resource.  t is the fully qualified type token and name is the "name" part
     * to use in creating a stable and globally unique URN for the object.  init is used to generate whatever children
     * will be parented to this component resource.  dependsOn is an optional list of other resources that this
     * resource depends on, controlling the order in which we perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param init The callback that will allocate child resources.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    constructor(t: string, name: string, props: ComputedValues,
                init: () => void | ComputedValues | undefined, dependsOn?: Resource[]) {
        super();
        const values: void | ComputedValues | undefined = Resource.runInParentScope(this, init);
        // IDEA: in the future, it would be nice to split inputs and outputs in the Pulumi metadata.  This would let
        //     us display them differently.  That implies fairly sizable changes to the RPC interfaces, however, so
        //     for now we simply cram both values (outputs) and props (inputs) together into the same property bag.
        this.register(t, name, false, Object.assign({}, values, props), dependsOn);
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

