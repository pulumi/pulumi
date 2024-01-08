import { PolicyPack, validateResourceOfType } from "@pulumi/policy";

new PolicyPack("typescript", {
    policies: [{
        name: "mandatory-policy-pack",
        description: "Failing mandatory policy pack for testing",
        enforcementLevel: "mandatory",
        validateStack: (stack, reportViolation) => {
	    reportViolation("mandatory-policy-pack");
        },
    }],
});
