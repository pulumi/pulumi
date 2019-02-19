// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";
import uuidv4 = require("uuid/v4");

export class Provider implements dynamic.ResourceProvider {
    public static readonly instance = new Provider();

    public async check(olds: any, news: any): Promise<dynamic.CheckResult> {
        return {
            inputs: news,
        };
    }

    public async diff(id: pulumi.ID, olds: any, news: any): Promise<dynamic.DiffResult> {
        if (olds.state !== news.state) {
            return {
                changes: true,
                replaces: ["state"],
                deleteBeforeReplace: news.noDBR ? false : true,
            };
        }

        if (olds.noReplace !== news.noReplace) {
            return {
                changes: true,
            }
        }

        return {
            changes: false,
        };
    }

    public async create(inputs: any): Promise<dynamic.CreateResult> {
        return {
            id: uuidv4(),
            outs: inputs,
        };
    }
}

export class Resource extends pulumi.dynamic.Resource {
    public uniqueKey?: pulumi.Output<number>;
    public state: pulumi.Output<number>;
    public noReplace?: pulumi.Output<number>;

    constructor(name: string, props: ResourceProps, opts?: pulumi.CustomResourceOptions) {
        super(Provider.instance, name, props, opts);
    }
}

export interface ResourceProps {
    readonly uniqueKey?: pulumi.Input<number>;
    readonly state: pulumi.Input<number>;
    readonly noReplace?: pulumi.Input<number>;
    readonly noDBR?: pulumi.Input<boolean>;
}
