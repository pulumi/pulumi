// Copyright 2025, Pulumi Corporation.  All rights reserved.


import { PolicyPack, ResourceValidationArgs } from "@pulumi/policy";
import { log } from "console";

new PolicyPack("config", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "allowed",
            description: "Verifies properties",
            enforcementLevel: "mandatory",
            validateResource: (args : ResourceValidationArgs, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                let config = args.getConfig<{ value: boolean }>();


                if (args.props.value !== config.value) {
                    reportViolation("Property was " + args.props.value);
                }
            },
        },
    ],
});
