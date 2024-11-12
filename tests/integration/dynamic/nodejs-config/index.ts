// Copyright 2016-2023, Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi";

class CustomResource extends pulumi.dynamic.Resource {
    public readonly authenticated!: pulumi.Output<string>;
    public readonly color!: pulumi.Output<string>;

    constructor(name: string, opts?: pulumi.ResourceOptions) {
        super(
            new DummyResourceProvider(),
            name,
            {
                authenticated: undefined,
                color: undefined
            },
            opts,
            "custom-provider",
            "CustomResource",
        );
    }
}

class DummyResourceProvider implements pulumi.dynamic.ResourceProvider {
    private password: string;
    private color: string;

    async configure(req: pulumi.dynamic.ConfigureRequest): Promise<any> {
        this.password = req.config.require("password");
        this.color = req.config.get("colors:banana") ?? "blue";
    }

    async create(props: any): Promise<pulumi.dynamic.CreateResult> {
        return {
            id: "resource-id",
            outs: {
                authenticated: this.password === "s3cret" ? "200" : "401",
                color: this.color,
            },
        };
    }
}

const resource = new CustomResource("resource-name");

export const authenticated = resource.authenticated;
export const color = resource.color;
