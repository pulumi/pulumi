import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const source = new nestedobject.Container("source", {inputs: [
    "a",
    "b",
]});
const sink = new nestedobject.Container("sink", {inputs: source.details.apply(details => details.map(__item => __item.value))});
