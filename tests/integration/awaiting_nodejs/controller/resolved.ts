import { Awaiting, RequiresKnown } from "./resources";

const stage = new Awaiting("stage", {
    id: "release",
    generation: 2,
    parts: { source: { ref: "source:review", value: "commit" } },
    artifacts: {},
});
new RequiresKnown("member", stage.release.apply(release => release.parts.source.ref));

const production = new Awaiting("production", {
    id: "production-release",
    generation: 3,
    parts: { source: { ref: "source:production", value: "commit" } },
    artifacts: {},
});
new RequiresKnown("production-application", production.release.apply(release => release.parts.source.ref));
