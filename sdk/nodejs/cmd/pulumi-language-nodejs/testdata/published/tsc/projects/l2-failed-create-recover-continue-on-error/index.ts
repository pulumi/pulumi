import * as pulumi from "@pulumi/pulumi";
import * as fail_on_create from "@pulumi/fail_on_create";
import * as simple from "@pulumi/simple";

const failing = new fail_on_create.Resource("failing", {value: false});
export const recovered = pulumi.recover(failing.urn, err => ((error) => `recovered: ${error}`)(err instanceof Error ? err.message : String(err)));
const recovered_value = new simple.Resource("recovered_value", {value: pulumi.recover(failing.value, err => ((error) => error != "")(err instanceof Error ? err.message : String(err)))});
const independent = new simple.Resource("independent", {value: true});
