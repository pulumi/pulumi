import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const container = new nestedobject.Container("container", {inputs: [
    "alpha",
    "bravo",
]});
const mapContainer = new nestedobject.MapContainer("mapContainer", {tags: {
    k1: "charlie",
    k2: "delta",
}});
// A resource that ranges over a computed list
const listOutput: nestedobject.Target[] = [];
container.details.apply(rangeBody => {
    for (const range of rangeBody.map((v, k) => ({key: k, value: v}))) {
        listOutput.push(new nestedobject.Target(`listOutput-${range.key}`, {name: range.value.value}));
    }
});
// A resource that ranges over a computed map
const mapOutput: nestedobject.Target[] = [];
mapContainer.tags.apply(rangeBody => {
    for (const range of Object.entries(rangeBody).sort().map(([k, v]) => ({key: k, value: v}))) {
        mapOutput.push(new nestedobject.Target(`mapOutput-${range.key}`, {name: `${range.key}=>${range.value}`}));
    }
});
