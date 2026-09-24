import * as pulumi from "@pulumi/pulumi";
import * as fail_on_create from "@pulumi/fail_on_create";
import * as read from "@pulumi/read";

const failing = new fail_on_create.Resource("failing", {value: false});
const res = read.Resource.get("res", "existing-id", {lookup: "existing-key"}, {
    dependsOn: [failing],
});
const res_prop_dep = read.Resource.get("res_prop_dep", "existing-id", {lookup: pulumi.interpolate`existing-key-${failing.value}`});
export const readValue = res.value;
export const readPropDepValue = res_prop_dep.value;
