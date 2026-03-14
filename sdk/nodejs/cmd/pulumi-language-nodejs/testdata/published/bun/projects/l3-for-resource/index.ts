import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const source = new nestedobject.Container("source", {inputs: [
    "a",
    "b",
    "c",
]});
// for over list<object> output
const receiver = new nestedobject.Receiver("receiver", {details: source.details.apply(details => details.map(detail => ({
    key: detail.key,
    value: detail.value,
})))});
// for over list<string> output
const fromSimple = new nestedobject.Container("fromSimple", {inputs: source.details.apply(details => details.map(detail => (detail.value)))});
// for producing a map
const mapped = new nestedobject.MapContainer("mapped", {tags: source.details.apply(details => details.reduce((__obj, detail) => ({ ...__obj, [detail.key]: detail.value }), {}))});
