import * as pulumi from "@pulumi/pulumi";
import * as fail_on_create from "@pulumi/fail_on_create";
import * as read from "@pulumi/read";

const failing = new fail_on_create.Resource("failing", {value: false});
const res = read.Resource.get("res", "existing-id", {lookup: "existing-key"}, {
    dependsOn: [failing],
});
export const readValue = res.value;
