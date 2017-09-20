// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as runtime from "./runtime";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

// Resource represents a class whose CRUD operations are implemented by a provider plugin.
export abstract class Resource {
    // urn is the stable logical URN used to distinctly address a resource, both before and after deployments.
    public readonly urn: Promise<URN>;
    // id is the provider-assigned unique ID for this resource.  It is set during deployments and may be missing
    // (undefined) during planning phases.
    public readonly id: Computed<ID>;

    // creates and registers a new resource object.  t is the fully qualified type token and name is the "name" part
    // to use in creating a stable and globally unique URN for the object.  dependsOn is an optional list of other
    // resources that this resource depends on, controlling the order in which we perform resource operations.
    constructor(t: string, name: string, props: ComputedValues, dependsOn?: Resource[]) {
        if (t === undefined || t === "") {
            throw new Error("Missing resource type argument");
        }
        if (name === undefined || name === "") {
            throw new Error("Missing resource name argument (for URN creation)");
        }

        // Now kick off the resource registration.  If we are actually performing a deployment, this resource's
        // properties will be resolved asynchronously after the operation completes, so that dependent computations
        // resolve normally.  If we are just planning, on the other hand, values will never resolve.
        runtime.registerResource(this, t, name, props, dependsOn);
    }
}

// Computed is a property output for a resource.  It is just a promise that also permits undefined values.  The
// undefined values are used during planning, when the actual final value of a resource may not yet be known.
export type Computed<T> = Promise<T | undefined>;

// ComputedValue is a property input for a resource.  It may be a promptly available T or a promise for one.
export type ComputedValue<T> = T | undefined | Computed<T>;

// ComputedValues is a map of property name to optional property input, one for each resource property value.
export type ComputedValues = {[key: string]: ComputedValue<any> | undefined};

