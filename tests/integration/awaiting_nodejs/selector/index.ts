import { Awaiting, RequiresKnown } from "./resources";

const stage = new Awaiting("stage", {
    id: "release",
    generation: 1,
    parts: { source: { ref: "commit-sha", value: "commit-sha" } },
    artifacts: {},
});
const member = new RequiresKnown("member", stage.release.apply(release => release.parts.source.ref));
new RequiresKnown("consumer", member.outputOnly);

new Awaiting("production", {
    id: "production-release",
    generation: 1,
    parts: { source: { ref: "commit-production", value: "commit-production" } },
    artifacts: {},
});
