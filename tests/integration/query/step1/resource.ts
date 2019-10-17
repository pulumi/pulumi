// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

let currentID = 0;

export class Provider implements pulumi.dynamic.ResourceProvider {
    public static readonly instance = new Provider();

    public async create(inputs: any) {
        return {
            id: (currentID++).toString(),
            outs: undefined,
        };
    }
}

export class Resource extends pulumi.dynamic.Resource {
    public isInstance(o: any): o is Resource {
        return o.__pulumiType === "pulumi-nodejs:dynamic:Resource";
    }

    constructor(name: string, props: pulumi.Inputs, opts?: pulumi.ResourceOptions) {
        super(Provider.instance, name, props, opts);
    }
}
