// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { PolicyPack } from "@pulumi/policy";

new PolicyPack("dryrun", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "dry",
            description: "Verifies properties are true on dryrun",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                if (pulumi.runtime.isDryRun()) {
                    if (!args.props.value) {
                        reportViolation("This is a test error");
                    }
                }
            },
        },

    ],
});
