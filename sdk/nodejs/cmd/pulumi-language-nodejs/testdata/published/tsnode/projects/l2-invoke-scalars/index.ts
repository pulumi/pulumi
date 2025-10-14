import * as pulumi from "@pulumi/pulumi";
import * as scalar_returns from "@pulumi/scalar-returns";

export const secret = scalar_returns.invokeSecretOutput({
    value: "goodbye",
});
export const array = scalar_returns.invokeArrayOutput({
    value: "the word",
});
export const map = scalar_returns.invokeMapOutput({
    value: "hello",
});
