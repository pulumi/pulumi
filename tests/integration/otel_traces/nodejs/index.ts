// Copyright 2026, Pulumi Corporation. All rights reserved.

import * as pulumi from "@pulumi/pulumi";

if (new pulumi.Config().require("failure") === "program") {
    throw new Error("intentional program failure");
}

class FailsOnCreate extends pulumi.CustomResource {
    constructor(name: string) {
        super("testprovider:index:FailsOnCreate", name, {});
    }
}

new FailsOnCreate("failure");
