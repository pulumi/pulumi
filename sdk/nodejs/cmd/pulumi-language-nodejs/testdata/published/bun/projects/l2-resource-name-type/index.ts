import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const res1 = new simple.Resource("res1", {value: true});
export const name = pulumi.resourceName(res1);
export const type = pulumi.resourceType(res1);
