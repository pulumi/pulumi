import { Awaiting, RequiresKnown } from "./resources";

const stage = new Awaiting("stage", {
    ready: false,
    version: "v1",
    release: { parts: { source: { value: "commit-v1" } } },
});
new RequiresKnown("member", {
    source: stage.release.apply(release => release.parts.source.value),
    outputOnly: stage.outputOnly,
});
