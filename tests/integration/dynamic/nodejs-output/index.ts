// Copyright 2025-2025, Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi";

class ResourceProvider implements pulumi.dynamic.ResourceProvider {
    async create(props: any): Promise<pulumi.dynamic.CreateResult> {
        console.log("message from provider")
        return {
            id: "resource-id",
            outs: {},
        };
    }
}

class CustomResource extends pulumi.dynamic.Resource {
    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super(new ResourceProvider(), name, {}, opts, "custom-provider", "CustomResource");
    }
}

const resource = new CustomResource("resource-name");
