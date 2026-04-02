import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const res1 = new simple.Resource("res1", {value: true});
const res2 = new simple.Resource("res2", {value: res1.value.apply(value => !value)});
