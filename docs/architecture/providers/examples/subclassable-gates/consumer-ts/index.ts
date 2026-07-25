// A rollout program that gates each environment behind a typed gate. The point: MetricGate and
// ApprovalGate are different gate *types* with different checks and wakes, but they are both
// `Gate`s -- so the rollout treats them uniformly, and every one of them inherits the base
// suspend/resume/status/`explain` machinery for free.

import * as pulumi from "@pulumi/pulumi";
import * as delivery from "@pulumi/delivery";

// Staging opens automatically while the error rate stays healthy; it resumes on a metric poll.
const staging = new delivery.MetricGate("staging", {
    until: false, // late-bound: the rollout driver flips this once the query is under threshold
    query: "sum:app.errors{env:staging}.as_rate()",
    threshold: 0.01,
    wake: { kind: "metric", params: { query: "sum:app.errors{env:staging}.as_rate()" } },
});

// Prod waits for a human; it resumes on an approval event.
const prod = new delivery.ApprovalGate("prod", {
    until: false,
    approvers: ["platform-oncall", "release-captain"],
    instructions: "Confirm staging soak looks clean before promoting to prod.",
    wake: { kind: "approval", params: { policy: "prod-release" } },
});

// Because both are `Gate`s, the rollout can hold them in one list and drive them uniformly.
// `explain()` is defined once on the base and inherited by every subclass -- no gate reimplements it.
const gates: delivery.Gate[] = [staging, prod];
export const holds = pulumi.all(gates.map((g) => g.explain().reason));

export const stagingStatus = staging.status;      // inherited output
export const prodResumesWhen = prod.resumeWhen;   // inherited output
