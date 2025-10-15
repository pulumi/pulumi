import * as pulumi from "@pulumi/pulumi";
import * as large from "@pulumi/large";

const res = new large.String("res", {value: "hello world"});
export const output = res.value;
