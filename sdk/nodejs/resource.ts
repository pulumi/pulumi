// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { registerResource } from "./runtime/resource";
import { getRootResource } from "./runtime/settings";

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
     * Creates and registers a new resource object.  t is the fully qualified type token and name is the "name" part
     * to use in creating a stable and globally unique URN for the object.  dependsOn is an optional list of other
     * resources that this resource depends on, controlling the order in which we perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param custom True to indicate that this is a custom resource, managed by a plugin.
     * @param props The arguments to use to populate the new resource.
     * @param parent An optional parent resource to which this resource belongs.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    constructor(t: string, name: string, custom: boolean, props?: ComputedValues,
                parent?: Resource, dependsOn?: Resource[]) {
        if (!t) {
            throw new Error("Missing resource type argument");
        }
        if (!name) {
            throw new Error("Missing resource name argument (for URN creation)");
        }

        // If there wasn't an explicit parent, and a root resource exists, parent to that.
        if (!parent) {
            parent = getRootResource();
        }

        // Now kick off the resource registration.  If we are actually performing a deployment, this resource's
        // properties will be resolved asynchronously after the operation completes, so that dependent computations
        // resolve normally.  If we are just planning, on the other hand, values will never resolve.
        registerResource(this, t, name, custom, props, parent, dependsOn);
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
     * @param parent An optional parent resource to which this resource belongs.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    constructor(t: string, name: string, props?: ComputedValues, parent?: Resource, dependsOn?: Resource[]) {
        super(t, name, true, props, parent, dependsOn);
    }
}

/**
 * ComponentResource is a resource that aggregates one or more other child resources into a higher level abstraction.
 * The component resource itself is a resource, but does not require custom CRUD operations for provisioning.
 */
export class ComponentResource extends Resource {
    /**
     * Creates and registers a new component resource.  t is the fully qualified type token and name is the "name" part
     * to use in creating a stable and globally unique URN for the object. parent is the optional parent for this
     * component, and dependsOn is an optional list of other resources that this resource depends on, controlling the
     * order in which we perform resource operations.
     *
     * @param t The type of the resource.
     * @param name The _unqiue_ name of the resource.
     * @param props The arguments to use to populate the new resource.
     * @param parent An optional parent resource to which this resource belongs.
     * @param dependsOn Optional additional explicit dependencies on other resources.
     */
    constructor(t: string, name: string, props?: ComputedValues, parent?: Resource, dependsOn?: Resource[]) {
        super(t, name, false, props, parent, dependsOn);
    }

    // recordOutput sets a property named key to the value val in this component's output properties.
    protected recordOutput(key: string, val: ComputedValue<any>): void {
        // TODO[pulumi/pulumi#340]: communicate outputs back to the engine via RPC so that it can record them
        //     inside of the resulting checkpoint file.
    }

    // recordOutputs sets all object keys and values from obj as properties in this component's output properties.
    protected recordOutputs(obj: ComputedValues): void {
        // TODO[pulumi/pulumi#340]: communicate outputs back to the engine via RPC so that it can record them
        //     inside of the resulting checkpoint file.
    }
}

/**
 * Computed is a property output for a resource.  It is just a promise that also permits undefined values.  The
 * undefined values are used during planning, when the actual final value of a resource may not yet be known.
 */
export type Computed<T> = Promise<T | undefined>;

/**
 * ComputedValue is a property input for a resource.  It may be a promptly available T or a promise for one.
 */
export type ComputedValue<T> = T | undefined | Promise<T | undefined>;

/**
 * ComputedValues is a map of property name to optional property input, one for each resource property value.
 */
export type ComputedValues = { [key: string]: ComputedValue<any> };
