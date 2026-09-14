import { Awaiting, RequiresKnown } from "./resources";

const stage = new Awaiting("stage", {
    id: "release",
    generation: 2,
    parts: { source: { ref: "part:/source", value: "commit-sha" } },
    artifacts: {},
});
const member = new RequiresKnown("member", stage.release.apply(release => release.parts.source.ref));
new RequiresKnown("consumer", member.outputOnly);

new Awaiting("production", {
    id: "production-release",
    generation: 2,
    parts: { source: { ref: "part:/production", value: "commit-production" } },
    artifacts: {},
}, false);
