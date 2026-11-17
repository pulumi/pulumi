import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";

const r = new output.Resource("r", {value: 1});
export const wrapped = pulumi.secret(r.output);
