import * as pulumi from "@pulumi/pulumi";
import * as read from "@pulumi/read";
import * as simple from "@pulumi/simple";

const src = new simple.Resource("src", {value: true});
const res = read.Resource.get("res", "existing-id", {lookup: "existing-key"}, {
    dependsOn: [src],
});
export const resourceUrn = res.urn;
export const resourceId = res.id;
export const lookup = res.lookup;
export const value = res.value;
