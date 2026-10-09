import * as pulumi from "@pulumi/pulumi";
import * as fail_on_create from "@pulumi/fail_on_create";
import * as simple from "@pulumi/simple";

const failing = new fail_on_create.Resource("failing", {value: false});
const independent = new simple.Resource("independent", {value: true});
export const failingValue = failing.value;
