import { PolicyPack } from "@pulumi/policy";

new PolicyPack("bun", {
    policies: [{
        name: "mandatory-policy-pack",
        description: "Failing mandatory policy pack for testing",
        enforcementLevel: "mandatory",
        validateStack: (stack, reportViolation) => {
            reportViolation("mandatory-policy-pack");
        },
    }],
});
