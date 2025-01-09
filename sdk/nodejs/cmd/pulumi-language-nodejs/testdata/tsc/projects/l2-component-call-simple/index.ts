import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

const component1 = new component.ComponentCallable("component1", {value: "bar"});
export const from_identity = component1.identity().apply(call => call.result);
export const from_prefixed = component1.prefixed(({
    prefix: "foo-",
})).apply(call => call.result);
