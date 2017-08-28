// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Property, PropertyValue } from "./property";
import * as runtime from "./runtime";

export type ID = string;  // a provider-assigned ID.
export type URN = string; // an automatically generated logical URN, used to stably identify resources.

// Resource represents a class whose CRUD operations are implemented by a provider plugin.
export abstract class Resource {
    // id is the provider-assigned unique ID for this object.  It is set during deployments.
    public readonly id: Property<ID>;
    // urn is the stable logical URN used to distinctly address an object, both before and after deployments.
    public readonly urn: Property<URN>;

    // creates and registers a new resource object.  t is the fully qualified type token and name is the "name" part
    // to use in creating a stable and globally unique URN for the object.
    constructor(t: string, name: string, props: {[key: string]: PropertyValue<any>}) {
        if (t === undefined || t === "") {
            throw new Error("Missing resource type argument");
        }
        if (name === undefined || name === "") {
            throw new Error("Missing resource name argument (for URN creation)");
        }

        // Now kick off the resource registration.  If we are actually performing a deployment, this resource's
        // properties will be resolved asynchronously after the operation completes, so that dependent computations
        // resolve normally.  If we are just planning, on the other hand, values will never resolve.
        runtime.registerResource(this, t, name, props);
    }
}

