import { Awaiting, RequiresKnown } from "./resources";

const test = new Awaiting("test", {
    ready: false,
    version: "v2",
    release: { parts: { source: { value: "commit-v2" } } },
});
const verify = new RequiresKnown("verify", {
    source: test.release.apply(release => release.parts.source.value),
    outputOnly: test.outputOnly,
}, { dependsOn: [test] });
const production = new Awaiting("production", {
    ready: false,
    version: "v2",
    release: test.release,
}, { dependsOn: [test] });
new RequiresKnown("application", {
    source: production.release.apply(release => release.parts.source.value),
    outputOnly: verify.outputOnly,
}, { dependsOn: [production, verify] });
