
import { PolicyPack } from "@pulumi/policy";

new PolicyPack("invalid-policy", {
    policies: [
        {
            name: "all",
            description: "Invalid policy name.",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => { throw new Error("Should never run."); },
            remediateResource: (args) => { throw new Error("Should never run."); },
        },
    ],
});