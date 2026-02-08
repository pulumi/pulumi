// Copyright 2026, Pulumi Corporation.  All rights reserved.


import { PolicyPack, ResourceValidationArgs } from "@pulumi/policy";

new PolicyPack("stack-tags", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "allowed",
            description: "Verifies property equals the stack tag value",
            enforcementLevel: "mandatory",
            validateResource: (args : ResourceValidationArgs, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                const tag = args.stackTags.get("value");
                if (tag === undefined) {
                    reportViolation("Stack tag 'value' is required");
                    return;
                }

                let expected = JSON.parse(tag);

                if (args.props.value !== expected) {
                    reportViolation("Property was " + args.props.value);
                }
            },
        },
    ],
});
