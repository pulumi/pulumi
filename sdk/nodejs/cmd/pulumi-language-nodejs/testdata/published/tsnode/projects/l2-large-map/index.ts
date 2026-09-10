import * as pulumi from "@pulumi/pulumi";
import * as large from "@pulumi/large";

const res = new large.Map("res", {
    value: "leaf",
    depth: 300,
});
export const output = res.value;
