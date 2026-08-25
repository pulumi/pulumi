// Copyright 2026, Pulumi Corporation.  All rights reserved.

import { PolicyPack } from "@pulumi/policy";

new PolicyPack("analyze-stack", {
    enforcementLevel: "mandatory",
    policies: [
        {
            name: "stack-size",
            description: "Stack must contain at most one simple:index:Resource",
            enforcementLevel: "mandatory",
            validateStack: (args, reportViolation) => {
                const count = args.resources.filter(r => r.type === "simple:index:Resource").length;
                if (count > 1) {
                    reportViolation("Found an extra simple:index:Resource");
                }
            },
        },
    ],
});
