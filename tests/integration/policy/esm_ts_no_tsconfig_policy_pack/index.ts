import policy from "@pulumi/policy";

// Top-level await only works if the policy pack loads as an ES module.
const loadedAs: string = await Promise.resolve("esm");

new policy.PolicyPack("esm-ts-no-tsconfig", {
    policies: [{
        name: "report-module-type",
        description: "Reports how the policy pack was loaded",
        enforcementLevel: "mandatory",
        validateStack: (stack, reportViolation) => {
            reportViolation(`loaded-as=${loadedAs}`);
        },
    }],
});
