// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
import * as pulumi from "@pulumi/pulumi";

let currentID = 0;

export class Provider implements pulumi.dynamic.ResourceProvider {
    public static readonly instance = new Provider();

    constructor() {}

    public async create(inputs: any) {
        return {
            id: (currentID++).toString(),
            outs: inputs,
        };
    }

    public async delete(id: pulumi.ID, props: any) {}

    public async diff(id: pulumi.ID, olds: any, news: any) { return {}; }

    public async update(id: pulumi.ID, olds: any, news: any) {
        return news;
    }
}

export class Resource extends pulumi.dynamic.Resource {
    constructor(name: string, props: ResourceProps, opts?: pulumi.ResourceOptions) {
        super(Provider.instance, name, props, opts);
    }
}

export interface ResourceProps {
    state?: any; // arbitrary state bag that can be updated without replacing.
}
