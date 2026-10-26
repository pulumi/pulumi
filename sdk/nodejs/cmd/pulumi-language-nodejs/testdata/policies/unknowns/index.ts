// Copyright 2026, Pulumi Corporation.  All rights reserved.

import { PolicyPack } from "@pulumi/policy";

// The @pulumi/policy runtime automatically reports the policy as not-applicable
// when a validation function reads an unknown value, so we can access the
// property directly and rely on that lifting for unknown and secret-unknown
// inputs. Secret-known inputs are unwrapped and surfaced as their underlying
// value.
new PolicyPack("unknowns", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "report",
            description: "Reports what the analyzer saw",
            enforcementLevel: "advisory",
            validateResource: (args, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }
                const value = (args.props as { value: boolean }).value;
                if (value) {
                    reportViolation("value is true");
                } else {
                    reportViolation("value is false");
                }
            },
        },
    ],
});
