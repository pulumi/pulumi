import * as pulumi from "@pulumi/pulumi";
import * as callreturnsprovider from "@pulumi/callreturnsprovider";

const component1 = new callreturnsprovider.ComponentCallable("component1", {});
export const from_identity = component1.identity(({
    value: "bar",
}));
