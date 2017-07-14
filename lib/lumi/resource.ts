// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

export type PID = string;
export type UPN = string;

// Resource represents a class whose CRUD operations are implemented by a provider plugin.
export abstract class Resource {
    public readonly pid: PID; // the provider-assigned ID (initialized by the runtime).
    public readonly upn: UPN; // the Universal Pulumi Name (initialized by the runtime).
}

// NamedResource is a kind of resource that has a friendly resource name associated with it.
export abstract class NamedResource extends Resource {
    public readonly name: string;

    constructor(name: string) {
        super();
        if (name === undefined || name === "") {
            throw new Error("Named resources must have a name");
        }
        this.name = name;
    }
}

