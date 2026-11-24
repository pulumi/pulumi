import * as pulumi from "@pulumi/pulumi";
import * as read from "@pulumi/read";

const src = new read.Resource("src", {value: true});
const res = read.Resource.get("res", src.id, {lookup: "existing-key"});
export const resourceUrn = res.urn;
export const resourceId = res.id;
export const lookup = res.lookup;
export const value = res.value;
