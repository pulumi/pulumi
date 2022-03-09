// Copyright 2016-2021, Pulumi Corporation.  All rights reserved

import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";

import { Slow } from "./slow";

// The component has a child resource that takes a long time to be created.
// We want to ensure the component's slow child resource will actually be created when the
// component is created inside `construct`.

class Component extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);
        // Create a child resource that takes a long time in the provider to be created.
        new Slow(`child-${name}`, {parent: this});
    }
}

class Provider implements provider.Provider {
    public readonly version = "0.0.1";

    construct(name: string, type: string, inputs: pulumi.Inputs,
              options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        if (type != "testcomponent:index:Component") {
            throw new Error(`unknown resource type ${type}`);
        }

        // Create the component with a slow child resource.
        const component = new Component(name, options);
        return Promise.resolve({
            urn: component.urn,
            state: {},
        });
    }
}

export function main(args: string[]) {
    return provider.main(new Provider(), args);
}

main(process.argv.slice(2));
