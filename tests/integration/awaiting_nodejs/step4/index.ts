import { Awaiting, RequiresKnown } from "./resources";

const stage = new Awaiting("stage", {
    ready: true,
    version: "v2",
    release: { parts: { source: { value: "commit-v2" } } },
});
new RequiresKnown("member", {
    source: stage.release.apply(release => release.parts.source.value),
    outputOnly: stage.outputOnly,
});
