import * as pulumi from "@pulumi/pulumi";
import * as math from "./math";

let config = new pulumi.Config("simple");
let w = Number(config.require("w")), x = Number(config.require("x")), y = Number(config.require("y"));
let sum = new math.Add("sum", x, y);
let square = new math.Mul("square", sum.sum, sum.sum);
let diff = new math.Sub("diff", square.product, w);
let divrem = new math.Div("divrem", diff.difference, sum.sum);
let result = new math.Add("result", divrem.quotient, divrem.remainder);
export let outputSum: pulumi.Output<number> = result.sum;
result.sum.apply(result => {
    console.log(`((x + y)^2 - w) / (x + y) + ((x + y)^2 - w) %% (x + y) = ${result}`);
});