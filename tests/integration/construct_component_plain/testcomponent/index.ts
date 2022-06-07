// Copyright 2016-2021, Pulumi Corporation.  All rights reserved

import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";
import * as provider from "@pulumi/pulumi/provider";

let currentID = 0;

class Resource extends dynamic.Resource {
    constructor(name: string, opts?: pulumi.CustomResourceOptions) {
        const provider = {
            create: async (inputs: any) => {
                return {
                    id: (currentID++).toString(),
                    outs: undefined,
                };
            },
        };
        super(provider, name, {}, opts);
    }
}

interface ComponentArgs {
    children?: number;
}

class Component extends pulumi.ComponentResource {
    constructor(name: string, args?: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);
        const children = args?.children ?? 0;
        if (children <= 0) {
            return;
        }
        for (let i = 0; i < children; i++) {
            new Resource(`child-${name}-${i+1}`, {parent: this});
        }
    }
}

class Provider implements provider.Provider {
    public readonly version = "0.0.1";

    construct(name: string, type: string, inputs: pulumi.Inputs,
              options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        if (type != "testcomponent:index:Component") {
            throw new Error(`unknown resource type ${type}`);
        }

        const component = new Component(name, inputs, options);
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
