import * as pulumi from "@pulumi/pulumi";
import * as subpackage from "@pulumi/subpackage";

export const parameterValue = subpackage.doHelloWorldOutput({
    input: "goodbye",
}).apply(invoke => invoke.output);
