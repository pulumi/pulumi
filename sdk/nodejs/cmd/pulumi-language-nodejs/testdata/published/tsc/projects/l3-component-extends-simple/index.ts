import * as pulumi from "@pulumi/pulumi";
import * as inherit from "@pulumi/inherit";

const derived = new inherit.Derived("derived", {
    message: "hello",
    scale: 3,
});
export const baseOutput = derived.baseOutput;
export const derivedOutput = derived.derivedOutput;
