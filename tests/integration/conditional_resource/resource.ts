// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

let currentID = 0;

export class Provider implements pulumi.dynamic.ResourceProvider {
    public static readonly instance = new Provider();

    public readonly create: (inputs: any) => Promise<pulumi.dynamic.CreateResult>;

    constructor() {
        this.create = async (inputs: any) => {
            return {
                id: (currentID++).toString(),
                outs: {
                    arg: inputs.arg,
                    state: inputs.arg + " world",
                },
            };
        };
    }
}

export class Resource extends pulumi.dynamic.Resource {
    public readonly arg?: pulumi.Output<string>;
    public readonly state?: pulumi.Output<string>;

    constructor(name: string, props: ResourceProps, opts?: pulumi.ResourceOptions) {
        props["state"] = undefined
        super(Provider.instance, name, props, opts);
    }
}

export interface ResourceProps {
    arg?: pulumi.Input<string>
}
