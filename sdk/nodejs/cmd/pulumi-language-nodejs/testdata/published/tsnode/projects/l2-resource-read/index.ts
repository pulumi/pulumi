import * as pulumi from "@pulumi/pulumi";
import * as read from "@pulumi/read";

const res = read.Resource.get("res", "existing-id", {lookup: "existing-key"});
export const resourceId = res.id;
export const resourceUrn = res.urn;
export const lookup = res.lookup;
export const value = res.value;
