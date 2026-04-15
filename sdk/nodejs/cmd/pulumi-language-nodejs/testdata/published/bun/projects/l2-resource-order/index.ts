import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const res1 = new simple.Resource("res1", {value: true});
const localVar = res1.value;
const res2 = new simple.Resource("res2", {value: localVar});
export const out = res2.value;
