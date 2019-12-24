import * as pulumi from "@pulumi/pulumi";
import * as mycomponent from "./proxy";

// This is code the user would write to use `mycomponent` from the guest language.

const res = new mycomponent.MyComponent("n", {
    input1: Promise.resolve(24),
});

export const id2 = res.myid;
export const output1 = res.output1;
export const customResource = res.customResource; // TODO: This comes back as the `id` - not a live resource object.
export const innerComponent = res.innerComponent.data;
