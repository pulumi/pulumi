import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const container = new nestedobject.Container("container", {inputs: [
    "alpha",
    "bravo",
]});
const target: nestedobject.Target[] = [];
container.details.apply(rangeBody => {
    for (const range of rangeBody.map((v, k) => ({key: k, value: v}))) {
        target.push(new nestedobject.Target(`target-${range.key}`, {name: range.value.value}));
    }
});
