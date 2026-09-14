import { Awaiting } from "./resources";

new Awaiting("stage", {
    id: "release",
    generation: 2,
    parts: { source: { ref: "source:review", value: "commit" } },
    artifacts: {},
});

new Awaiting("production", {
    id: "",
    generation: 0,
    parts: {},
    artifacts: {},
});
