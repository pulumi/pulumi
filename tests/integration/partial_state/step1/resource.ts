// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";

export class Provider implements dynamic.ResourceProvider {
    public static readonly instance = new Provider();
    private id: number = 0;

    public async check(olds: any, news: any): Promise<dynamic.CheckResult> {
        return {
            inputs: news,
        };
    }

    public async create(inputs: any): Promise<dynamic.CreateResult> {
        let error: string | undefined = undefined;
        if (inputs.state === 4) {
            error = "state can't be 4";
        }

        return {
            id: (this.id++).toString(),
            outs: inputs,
            error: error,
        };
    }

    public async update(id: pulumi.ID, olds: any, news: any): Promise<dynamic.UpdateResult> {
        let error: string | undefined = undefined;
        if (news.state === 4) {
            error = "state can't be 4";
        }

        return {
            outs: news,
            error: error,
        };
    }
}

export class Resource extends dynamic.Resource {
    public readonly state: pulumi.Output<number>;

    constructor(name: string, num: pulumi.Input<number>, opts?: pulumi.ResourceOptions) {
        super(Provider.instance, name, { state: num }, opts);
    }
}
