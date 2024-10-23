// Copyright 2016-2023, Pulumi Corporation.

import * as pulumi from '@pulumi/pulumi'

class SimpleProvider implements pulumi.dynamic.ResourceProvider {
    async create(inputs: any, config: pulumi.dynamic.Config): Promise<pulumi.dynamic.CreateResult> {
        const password = config.require("password")
        return {
            id: "0", outs: {
                authenticated: password === "s3cret" ? "200" : "401",
            }
        }
    }
}

class SimpleResource extends pulumi.dynamic.Resource {
    readonly authenticated!: pulumi.Output<string>

    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super(new SimpleProvider(), name, { authenticated: undefined, capturedSecretApply: undefined, capturedSecretGet: undefined }, opts)
    }
}

const resource = new SimpleResource("resource")

export const authenticatedWithConfig = resource.authenticated
