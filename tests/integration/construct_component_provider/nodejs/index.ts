// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Provider extends pulumi.ProviderResource {
    public readonly message!: pulumi.Output<string>;

    constructor(name: string, message: string, opts?: pulumi.ResourceOptions) {
        super("testcomponent", name, { message }, opts);
    }
}

class Component extends pulumi.ComponentResource {
    public readonly message!: pulumi.Output<string>;

    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        const inputs = {
            message: undefined /*out*/,
        };
        super("testcomponent:index:Component", name, inputs, opts, true);
    }
}

const component = new Component("mycomponent", {
    providers: {
        "testcomponent": new Provider("myprovider", "hello world"),
    },
});

export const message = component.message;
