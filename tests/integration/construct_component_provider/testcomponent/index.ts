// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";

const version = "0.0.1";

class Provider extends pulumi.ProviderResource {
    public readonly message!: pulumi.Output<string>;

    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super("testcomponent", name, { "message": undefined }, opts);
    }
}

class Component extends pulumi.ComponentResource {
    public message!: pulumi.Output<string>;

    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);
    }

    protected async initialize(args: pulumi.Inputs) {
        const provider = this.getProvider("testcomponent::");
        if (!(provider instanceof Provider)) {
            throw new Error("provider is not an instance of Provider");
        }
        this.message = provider.message;
        this.registerOutputs({
            message: provider.message,
        });
        return undefined;
    }
}

class ProviderServer implements provider.Provider {
    public readonly version = version;

    constructor() {
        pulumi.runtime.registerResourcePackage("testcomponent", {
            version,
            constructProvider: (name: string, type: string, urn: string): pulumi.ProviderResource => {
                if (type !== "pulumi:providers:testcomponent") {
                    throw new Error(`unknown provider type ${type}`);
                }
                return new Provider(name, { urn });
            },
        });
    }

    async construct(name: string, type: string, inputs: pulumi.Inputs,
              options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        if (type != "testcomponent:index:Component") {
            throw new Error(`unknown resource type ${type}`);
        }

        const component = new Component(name, options);
        return {
            urn: component.urn,
            state: {
                message: component.message,
            },
        };
    }
}

export function main(args: string[]) {
    return provider.main(new ProviderServer(), args);
}

main(process.argv.slice(2));
