import { PolicyPack } from "@pulumi/policy";

new PolicyPack("pinned-node", {
    policies: [{
        name: "report-node-version",
        description: "Reports the Node.js version the pack runs on.",
        enforcementLevel: "mandatory",
        validateStack: (stack, reportViolation) => {
            reportViolation(`policy pack running on node ${process.version}`);
        },
    }],
});
