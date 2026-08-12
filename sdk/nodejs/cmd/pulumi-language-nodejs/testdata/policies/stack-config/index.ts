// Copyright 2025, Pulumi Corporation.  All rights reserved.


import { PolicyPack, ResourceValidationArgs } from "@pulumi/policy";

import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const value = config.requireBoolean("value");

new PolicyPack("stack-config", {
    enforcementLevel: "mandatory",
    policies: [
        {
            name: `validate-${value}`,
            description: `Verifies property is ${value}`,
            enforcementLevel: "mandatory",
            validateResource: (args : ResourceValidationArgs, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                if (args.props.value !== value) {
                    reportViolation("Property was " + args.props.value);
                }
            },
        },
    ],
});
