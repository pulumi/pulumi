// Copyright 2025, Pulumi Corporation.  All rights reserved.


import { PolicyPack } from "@pulumi/policy";

new PolicyPack("simple", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "truthiness",
            description: "Verifies properties are true",
            enforcementLevel: "advisory",
            validateResource: (args, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                if (!!args.props.value) {
                    reportViolation("This is a test warning");
                }
            },
        },
        {
            name: "falsiness",
            description: "Verifies properties are false",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                if (!args.props.value) {
                    reportViolation("This is a test error");
                }
            },
        },

    ],
});
