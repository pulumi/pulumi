// Copyright 2016-2021, Pulumi Corporation.  All rights reserved

import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";

import { Random } from "./random";

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
            new Random(`child-${name}-${i+1}`, { length: 10 }, {parent: this});
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
