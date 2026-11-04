import * as pulumi from "@pulumi/pulumi";
import * as fail_on_create from "@pulumi/fail_on_create";
import * as simple from "@pulumi/simple";

const independent = new simple.Resource("independent", {value: true});
export const recovered = pulumi.recover(fail_on_create.failOutput({
    value: independent.value,
}).message, err => ((error) => `recovered: ${error}`)(err instanceof Error ? err.message : String(err)));
const recovered_value = new simple.Resource("recovered_value", {value: pulumi.recover(fail_on_create.failOutput({
    value: independent.value,
}).value, err => ((error) => error != "")(err instanceof Error ? err.message : String(err)))});
