import * as pulumi from "@pulumi/pulumi";
import * as read_id_rewrite from "@pulumi/read-id-rewrite";
import * as simple from "@pulumi/simple";

const res = read_id_rewrite.Resource.get("res", "existing-id", {lookup: "existing-key"});
const src = new simple.Resource("src", {value: true});
const unknownRes = read_id_rewrite.Resource.get("unknownRes", src.id, {lookup: "another-key"});
export const resourceId = res.id;
export const lookup = res.lookup;
export const unknownResourceId = unknownRes.id;
export const unknownLookup = unknownRes.lookup;
