// Copyright 2024 Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";

class Provider implements provider.Provider {
    constructor(readonly version: string) {}

    async construct(name: string, type: string, inputs: pulumi.Inputs,
        options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        const typeName = type.split(":", 3)[2];
        return {
            urn: pulumi.createUrn(type, name),
            state: {
                "ITS_ALIVE": "IT'S ALIVE!",
            }
        }
    }
}

const prov = new Provider("0.0.1");
pulumi.provider.main(prov, process.argv.slice(2));
