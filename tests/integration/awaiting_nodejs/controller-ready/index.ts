import { Awaiting, RequiresKnown } from "./resources";

const stage = new Awaiting("stage", {
    id: "release",
    generation: 2,
    parts: { source: { ref: "source:review", value: "commit" } },
    artifacts: {},
});

new RequiresKnown("member", stage.release.apply(release => release.parts.source.ref));

