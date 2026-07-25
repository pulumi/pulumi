import * as pulumi from "@pulumi/pulumi";
import * as inheritderived from "@pulumi/inheritderived";

const derived = new inheritderived.DerivedComponent("derived", {
    message: "hello",
    scale: 7,
});
export const baseOutput = derived.baseOutput;
export const derivedOutput = derived.derivedOutput;
