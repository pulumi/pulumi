import * as pulumi from "@pulumi/pulumi";
import * as simple_invoke_with_scalar_return from "@pulumi/simple-invoke-with-scalar-return";

export const scalar = simple_invoke_with_scalar_return.myInvokeScalarOutput({
    value: "goodbye",
});
