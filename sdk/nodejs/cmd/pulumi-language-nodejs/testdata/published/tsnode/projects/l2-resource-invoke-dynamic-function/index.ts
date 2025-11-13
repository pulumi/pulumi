import * as pulumi from "@pulumi/pulumi";
import * as any_type_function from "@pulumi/any-type-function";

const localValue = "hello";
export const dynamic = any_type_function.dynListToDynOutput({
    inputs: [
        "hello",
        localValue,
        {},
    ],
}).apply(invoke => invoke.result);
