// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import { PolicyPack } from "@pulumi/policy";

new PolicyPack("validate-resource-test-policy", {
    policies: [
        {
            name: "dynamic-no-state-with-value-1",
            description: "Prohibits setting state to 1 on dynamic resources.",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type === "pulumi-nodejs:dynamic:Resource") {
                    if (args.props.state === 1) {
                        reportViolation("'state' must not have the value 1.")
                    }
                }
            },
        },
    ],
});
