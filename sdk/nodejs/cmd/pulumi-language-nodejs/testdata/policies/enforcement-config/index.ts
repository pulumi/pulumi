// Copyright 2025, Pulumi Corporation.  All rights reserved.


import { PolicyPack, ResourceValidationArgs } from "@pulumi/policy";

new PolicyPack("enforcement-config", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "false",
            description: "Verifies property is false",
            enforcementLevel: "advisory",
            validateResource: (args : ResourceValidationArgs, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                if (args.props.value) {
                    reportViolation("Property was " + args.props.value);
                }
            },
        },
    ],
});
