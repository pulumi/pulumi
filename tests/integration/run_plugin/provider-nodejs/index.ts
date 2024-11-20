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
	    state: {}
        }
    }

    async read(id: string, urn: string, props?: any): Promise<provider.ReadResult> {
	return {
	    id: id,
	    props: {
		"PULUMI_ROOT_DIRECTORY": process.env.PULUMI_ROOT_DIRECTORY,
		"PULUMI_PROGRAM_DIRECTORY": process.env.PULUMI_PROGRAM_DIRECTORY,
	    },
	}
    }
}

const prov = new Provider("0.0.1");
pulumi.provider.main(prov, process.argv.slice(2));
