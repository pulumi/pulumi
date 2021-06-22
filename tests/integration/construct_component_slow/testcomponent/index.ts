// Copyright 2016-2021, Pulumi Corporation.  All rights reserved

import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";
import * as provider from "@pulumi/pulumi/provider";

// The component has a child resource that takes a long time to be created.
// We want to ensure the component's slow child resource will actually be created when the
// component is created inside `construct`.

const CREATION_DELAY = 15 * 1000; // 15 second delay in milliseconds

let currentID = 0;

class SlowResource extends dynamic.Resource {
    constructor(name: string, opts?: pulumi.CustomResourceOptions) {
        const provider = {
            // Return the result after a delay to simulate a resource that takes a long time
            // to be created.
            create: async (inputs: any) => {
                await delay(CREATION_DELAY);
                return {
                    id: (currentID++).toString(),
                    outs: undefined,
                };
            },
        };
        super(provider, name, {}, opts);
    }
}

function delay(timeout: number): Promise<void> {
    return new Promise((resolve, reject) => {
        setTimeout(() => {
            resolve();
        }, timeout);
    });
}

class Component extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);
        // Create a child resource that takes a long time in the provider to be created.
        new SlowResource(`child-${name}`, {parent: this});
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
