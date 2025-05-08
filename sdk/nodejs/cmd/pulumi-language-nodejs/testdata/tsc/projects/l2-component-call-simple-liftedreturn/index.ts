import * as pulumi from "@pulumi/pulumi";
import * as componentreturnscalar from "@pulumi/componentreturnscalar";

const component1 = new componentreturnscalar.ComponentCallable("component1", {value: "bar"});
export const from_identity = component1.identity();
export const from_prefixed = component1.prefixed(({
    prefix: "foo-",
}));
