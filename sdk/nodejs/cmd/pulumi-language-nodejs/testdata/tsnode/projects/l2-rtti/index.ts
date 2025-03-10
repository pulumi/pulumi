import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const res = new simple.Resource("res", {value: true});
export const name = pulumi.resourceName(res);
export const type = pulumi.resourceType(res);
