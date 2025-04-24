import { PolicyPack, validateResourceOfType } from "@pulumi/policy";

new PolicyPack("typescript", {
    policies: [{
        name: "advisory-policy-pack",
        description: "Failing advisory policy pack for testing",
        enforcementLevel: "advisory",
        validateStack: (stack: any, reportViolation: any) => {
	    reportViolation("foobar");
        },
    }],
});
