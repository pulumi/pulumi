import * as pulumi from "@pulumi/pulumi";
import * as read from "@pulumi/read";

const failing = read.Resource.get("failing", "existing-id", {lookup: "fail"});
export const failingValue = failing.value;
export const failingLookup = failing.lookup;
