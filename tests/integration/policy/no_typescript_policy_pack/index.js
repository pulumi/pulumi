const { PolicyPack } = require("@pulumi/policy");

const tsNodeLoaded = Object.keys(require.cache).some((p) => /ts-node/.test(p));

new PolicyPack("no-typescript", {
    policies: [{
        name: "report-ts-node",
        description: "Reports whether ts-node was loaded",
        enforcementLevel: "mandatory",
        validateStack: (stack, reportViolation) => {
            reportViolation(`ts-node-loaded=${tsNodeLoaded}`);
        },
    }],
});
