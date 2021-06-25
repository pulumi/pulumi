// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";

class Component extends pulumi.ComponentResource {
    public readonly foo: pulumi.Output<string>;

    constructor(name: string, foo: pulumi.Input<string>, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);

        this.foo = pulumi.output(foo);

        this.registerOutputs({
            foo: this.foo,
        })
    }
}

class Provider implements provider.Provider {
    public readonly version = "0.0.1";

    construct(name: string, type: string, inputs: pulumi.Inputs,
              options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        if (type != "testcomponent:index:Component") {
            throw new Error(`unknown resource type ${type}`);
        }

       const foo = pulumi.output("").apply(a => {
           throw new Error("intentional error from within an apply");
           return a;
       });


        const component = new Component(name, foo);
        return Promise.resolve({
            urn: component.urn,
            state: {
                foo: component.foo
            },
        });
    }
}

export function main(args: string[]) {
    return provider.main(new Provider(), args);
}

main(process.argv.slice(2));
