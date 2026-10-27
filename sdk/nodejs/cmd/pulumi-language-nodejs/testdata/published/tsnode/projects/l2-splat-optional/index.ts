import * as pulumi from "@pulumi/pulumi";
import * as splatoptional from "@pulumi/splatoptional";

const source = new splatoptional.Container("source", {inputs: [
    "a",
    "b",
]});
const sink = new splatoptional.Container("sink", {inputs: source.details.apply(details => details.map(__item => __item.value)).apply(x =>x.map(item => item!))});
export const values = source.details.apply(details => details.map(__item => __item.value));
