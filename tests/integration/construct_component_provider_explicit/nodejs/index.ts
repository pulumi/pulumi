// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

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

class LocalComponent extends pulumi.ComponentResource {
    public readonly message: pulumi.Output<string>;

    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:index:LocalComponent", name, {}, opts);

        const component = new Component(`${name}-mycomponent`, { parent: this });
        this.message = component.message;
    }
}

const provider = new Provider("myprovider", "hello world")
const component = new Component("mycomponent", { provider });
const localComponent = new LocalComponent("mylocalcomponent", { providers: [provider] });

export const message = component.message;
export const nestedMessage = localComponent.message;
