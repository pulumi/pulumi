import * as pulumi from "@pulumi/pulumi";
import * as inherit from "@pulumi/inherit";

const derived = new inherit.Derived("derived", {
    message: "hi",
    scale: 1,
});
export const status = derived.getStatus().apply(call => call.status);
